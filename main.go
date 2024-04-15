package main

import (
	"context"
	"encoding/csv"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"strconv"

	"google.golang.org/api/option"
	"google.golang.org/api/youtube/v3"
)

var (
	logger      slog.Logger
	lvl         = new(slog.LevelVar)
	numVideo    int64
	durationReg = regexp.MustCompile(`^P((?P<year>\d+)Y)?((?P<month>\d+)M)?((?P<week>\d+)W)?((?P<day>\d+)D)?(T((?P<hour>\d+)H)?((?P<minute>\d+)M)?((?P<second>\d+)S)?)?$`)
)

func init() {
	lvl.Set(slog.LevelError)

	logger = *slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: lvl,
	}))
}

func itemValid(item youtube.Video) (bool, error) {
	valid := false
	// TODO handmade filter
	match := durationReg.FindStringSubmatch(item.ContentDetails.Duration)
	for i, name := range durationReg.SubexpNames() {
		part := match[i]
		if i == 0 || name == "" || part == "" {
			continue
		}
		val, err := strconv.Atoi(part)
		if err != nil {
			return false, err
		}
		switch name {
		case "year":
			return true, nil
		case "month":
			return true, nil
		case "week":
			return true, nil
		case "day":
			return true, nil
		case "hour":
			return true, nil
		case "minute":
			if val > 4 {
				return true, nil
			}
		case "second":
			// pass
		default:
			return false, fmt.Errorf("unknown field %s", name)
		}
	}
	return valid, nil
}

func getVideoInfoFromHandle(ctx context.Context, handle string) (map[string]youtube.Video, error) {
	svc, err := youtube.NewService(ctx, option.WithCredentialsFile("keyfile.json"))
	if err != nil {
		logger.Error("Error creating service", err)
		return nil, err
	}
	// Get Upload playlist id from Channel
	cresp, err := svc.Channels.List([]string{"contentDetails"}).ForHandle(handle).Do()
	if err != nil {
		logger.Error("Error Channels.List", slog.Any("err", err))
		return nil, err
	}
	logger.Debug("Channels.List", slog.Any("response", cresp))
	var channelId *string
	for _, item := range cresp.Items {
		logger.Debug("Channels.List", slog.Any("item", item))
		channelId = &item.ContentDetails.RelatedPlaylists.Uploads
		break
	}
	logger.Info("Found playlist", "id", *channelId)
	infos := make(map[string]youtube.Video)
	nextPageToken := ""
	for {
		// Get video Ids from Playlist
		req := svc.PlaylistItems.List([]string{"snippet"}).PlaylistId(*channelId).MaxResults(50)
		if nextPageToken != "" {
			logger.Debug("Paging", "pageToken", nextPageToken)
			req.PageToken(nextPageToken)
		}
		presp, err := req.Do()
		if err != nil {
			logger.Error("Error PlaylistItems.List", slog.Any("err", err))
			return nil, err
		}
		logger.Debug("PlaylistItems.List", slog.Any("response", presp))
		var videoIds []string
		for _, item := range presp.Items {
			logger.Debug("Playlists.List", slog.Any("Snippet", item.Snippet))
			logger.Debug("Playlists.List", slog.Any("ContentDetails", item.ContentDetails))
			videoIds = append(videoIds, item.Snippet.ResourceId.VideoId)
		}
		logger.Info("Found video ids", slog.Any("videos", videoIds))
		// Fetch video infos
		vresp, err := svc.Videos.List([]string{
			"snippet",
			"contentDetails",
			"statistics",
		}).Id(videoIds...).Do()
		if err != nil {
			logger.Error("Error Videos.List", slog.Any("err", err))
			return nil, err
		}
		for _, item := range vresp.Items {
			valid, _ := itemValid(*item)
			if !valid {
				continue
			}
			infos[item.Id] = *item
			logger.Info("Video", "info", infos[item.Id])
			if len(infos) >= int(numVideo) {
				presp.NextPageToken = ""
				break
			}
		}
		nextPageToken = presp.NextPageToken
		if nextPageToken == "" {
			break
		}
	}
	return infos, nil

}

func info2CSV(infos map[string]youtube.Video) error {
	w := csv.NewWriter(os.Stdout)
	w.Write([]string{"ID", "Title", "PublishedAt", "Duration", "ViewCount", "LikeCount", "CommentCount"})
	var records = make([][]string, 0)
	for _, item := range infos {
		records = append(records, []string{
			item.Id,
			item.Snippet.Title,
			item.Snippet.PublishedAt,
			item.ContentDetails.Duration,
			fmt.Sprintf("%d", item.Statistics.ViewCount),
			fmt.Sprintf("%d", item.Statistics.LikeCount),
			fmt.Sprintf("%d", item.Statistics.CommentCount),
		})

	}
	w.WriteAll(records)
	w.Flush()
	return nil
}

func main() {
	// Create Youtube Service
	ctx := context.Background()
	flag.Int64Var(&numVideo, "n", 10, "number of video to fetch")
	var verbose int
	flag.IntVar(&verbose, "v", 0, "verbose")
	flag.Parse()
	if verbose > 1 {
		lvl.Set(slog.LevelDebug)
	} else if verbose > 0 {
		lvl.Set(slog.LevelInfo)
	}
	handle := flag.Arg(0)
	if len(handle) < 1 {
		fmt.Print(("Usage: please enter a user handle."))
		return
	}
	infos, err := getVideoInfoFromHandle(ctx, handle)
	if err != nil {
		return
	}
	info2CSV(infos)

}
