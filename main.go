package main

import (
	"context"
	"encoding/csv"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"google.golang.org/api/option"
	"google.golang.org/api/youtube/v3"
)

var (
	logger    slog.Logger
	lvl       = new(slog.LevelVar)
	num_video int64
)

func init() {
	lvl.Set(slog.LevelError)

	logger = *slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: lvl,
	}))
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
			// TODO handmade filter
			if !strings.Contains(item.ContentDetails.Duration, "H") {
				continue
			}
			infos[item.Id] = *item
			logger.Info("Video", "info", infos[item.Id])
			if len(infos) >= int(num_video) {
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
	w.Write([]string{"ID", "Title", "PublishedAt", "Duration", "ViewCount", "LikeCount"})
	var records = make([][]string, 0)
	for _, item := range infos {
		records = append(records, []string{
			item.Id,
			item.Snippet.Title,
			item.Snippet.PublishedAt,
			item.ContentDetails.Duration,
			fmt.Sprintf("%d", item.Statistics.ViewCount),
			fmt.Sprintf("%d", item.Statistics.LikeCount),
		})

	}
	w.WriteAll(records)
	w.Flush()
	return nil
}

func main() {
	// Create Youtube Service
	ctx := context.Background()
	flag.Int64Var(&num_video, "n", 10, "number of video to fetch")
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
