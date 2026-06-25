package main

import (
	"fmt"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/mmcdole/gofeed"
)

func StartScheduler(s *discordgo.Session) {
	// Runs every 24 hours. (Tip: Change to 30 * time.Second during local testing!)
	ticker := time.NewTicker(24 * time.Hour) //time.NewTicker(24 * time.Hour)
	
	go func() {
		for {
			select {
			case <-ticker.C:
				runDailyTrackingJob(s)
			}
		}
	}()
}

func runDailyTrackingJob(s *discordgo.Session) {
	fmt.Printf("[%s] 🕒 Starting daily RSS tracking cycle...\n", time.Now().Format("15:04:05"))
	
	// 1. Load all tracking list names by passing an empty string
	metaData, err := LoadTrackingData("")
	if err != nil {
		fmt.Println("Scheduler error loading list names:", err)
		return
	}

	fp := gofeed.NewParser()

	// 2. Iterate through every tracking list found
	for _, listName := range metaData.Urls {
		// Load the full file details (ChannelID and target RSS URLs)
		listDetails, err := LoadTrackingData(listName)
		if err != nil {
			fmt.Printf("Error loading details for list %s: %v\n", listName, err)
			continue
		}

		// Skip if there's no valid channel ID or no URLs mapped
		if listDetails.ChannelID == "" || len(listDetails.Urls) == 0 {
			continue
		}

		// 3. Process every RSS URL in this specific list
		for _, url := range listDetails.Urls {
			feed, err := fp.ParseURL(url)
			if err != nil {
				fmt.Printf("Error parsing RSS feed (%s): %v\n", url, err)
				continue
			}

			// Look at the latest items in the feed
			for idx, item := range feed.Items {
				// Only fetch the newest few items so we don't spam the chat on day 1
				if idx >= 3 { 
					break 
				}

				// Extract a thumbnail if it exists in the feed item
				thumbnailURL := ""
				// Extract from the Media RSS extension map ("media" namespace -> "thumbnail" element)
				if mediaExtensions, exists := item.Extensions["media"]; exists {
					if thumbnails, ok := mediaExtensions["thumbnail"]; ok && len(thumbnails) > 0 {
						// Pull the "url" attribute from the first thumbnail found
						if urlAttr, typed := thumbnails[0].Attrs["url"]; typed {
							thumbnailURL = urlAttr
						}
					}
				}else if item.Image != nil {
					thumbnailURL = item.Image.URL
				} else if len(item.Extensions["media"]["content"]) > 0 {
					// Fallback handle for standard Media RSS thumbnails
					thumbnailURL = item.Extensions["media"]["content"][0].Attrs["url"]
				}

				// 4. Construct a Discord Embed with Name, Link, and Thumbnail
				embed := &discordgo.MessageEmbed{
					Title:       item.Title,
					Description: fmt.Sprintf("📰 New update from **%s**", feed.Title),
					URL:         item.Link,
					Color:       0x3498db, // Elegant blue color accent line
					Timestamp:   time.Now().Format(time.RFC3339),
				}

				// Attach thumbnail only if we found a valid image link
				if thumbnailURL != "" {
					embed.Image = &discordgo.MessageEmbedImage{
						URL: thumbnailURL,
					}
				}

				// 5. Send strictly to the saved channel ID
				_, err = s.ChannelMessageSendEmbed(listDetails.ChannelID, embed)
				if err != nil {
					fmt.Printf("Error sending embed to channel %s: %v\n", listDetails.ChannelID, err)
				}
			}

			// Wait 2 seconds
			time.Sleep(2 * time.Minute)
		}
	}
	fmt.Println("✅ Daily tracking cycle complete.")
}