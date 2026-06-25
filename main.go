package main

import (
	"fmt"
	"os"
	"regexp"
	"syscall"
	"strings"
	"os/signal"

	"github.com/joho/godotenv"
	"github.com/bwmarrin/discordgo"
)

// Define commands
var commands = []*discordgo.ApplicationCommand{
	{Name: "help", Description: "List the commands"},
	{
		Name:        "track",
		Description: "Add/Update the tracking list.",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "name",
				Description: "The name of the tracking list (e.g., world_news)",
				Required:    true,
			},
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "channel_id",
				Description: "The Channel ID in which the bot should post the links",
				Required:    true,
			},
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "urls",
				Description: "Comma-separated list of URLs",
				Required:    true,
			},
		},
	},
	{
		Name:        "tracking",
		Description: "View the tracking lists.",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "name",
				Description: "The name of the tracking list (e.g., world_news)",
			},
		},
	},
	{
		Name:        "delete",
		Description: "Delete the tracking lists.",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "name",
				Description: "The name of the tracking list (e.g., world_news)",
				Required:    true,
			},
		},
	},
}

// Define handlers
var commandHandlers = map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate){
	"help": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
	    helpText := "### 🤖 Command Guide\n" +
	        "Here is how to use the available commands:\n\n" +
	        
	        "➡️ **/track**\n" +
	        "Create or update a tracking list. You need to provide a **name, without space,** for your list and a **comma-separated list of RSS URLs**.\n" +
	        " *Example:* `/track name: world_news channel_id: 123 urls: https://www.reddit.com/r/worldnews/.rss`\n" +
	        " *Note:* This list is checked daily, and new updates will be posted automatically.\n\n" +
	        
	        "➡️ **/tracking**\n" +
	        "View your tracking lists.\n" +
	        " • Use it without arguments to see a quick list of all your tracking list **names**.\n" +
	        " • Provide a specific **name** to view all the URLs saved inside that list.\n\n" +
	        
	        "➡️ **/delete**\n" +
	        "Permanently delete a tracking list.\n" +
	        " • Provide the **name** of the list you want to remove.\n" +
	        " *Example:* `/delete name: world_news`"

	    s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
	        Type: discordgo.InteractionResponseChannelMessageWithSource,
	        Data: &discordgo.InteractionResponseData{
	            Content: helpText,
	            // Ephemeral means only the person who typed the command can see this help message.
	            // This prevents the help text from spamming the chat channel for everyone else!
	            Flags: discordgo.MessageFlagsEphemeral, 
	        },
	    })
	},
	"track": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		// Access the options sent by the user
		options := i.ApplicationCommandData().Options

		// Convert options array to a map for easy lookup
		optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption)
		for _, opt := range options {
			optionMap[opt.Name] = opt
		}

		// Extract the actual values
		listName := optionMap["name"].StringValue()
		channelID := optionMap["channel_id"].StringValue()
		rawUrls := optionMap["urls"].StringValue()

		// 1. Remove all spaces and special characters (Keep only a-z, A-Z, 0-9)
		reg, err := regexp.Compile("[^a-zA-Z0-9]+")
		if err != nil {
		    fmt.Println("Regex compilation error:", err)
		}
		listName = reg.ReplaceAllString(listName, "")

		// 2. If the user provided ONLY spaces/special characters, it will now be empty
		if listName == "" {
		    s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		        Type: discordgo.InteractionResponseChannelMessageWithSource,
		        Data: &discordgo.InteractionResponseData{
		            Content: "❌ Error: The list name must contain at least one letter or number!",
		            Flags:   discordgo.MessageFlagsEphemeral,
		        },
		    })
		    return
		}

		// 3. Trim the length to a maximum of 30 characters
		if len(listName) > 30 {
		    listName = listName[:30]
		}

		// Clean up the input: split by comma and trim extra spaces
		urlList := strings.Split(rawUrls, ",")
		for idx, url := range urlList {
			urlList[idx] = strings.TrimSpace(url)
		}
		
		err = SaveTrackingData(listName, channelID, urlList)
		if err != nil {
			fmt.Println("Error saving tracking data:", err)
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{Content: "❌ Failed to save tracking list."},
			})
			return
		}

		// Respond to the user confirming success
		responseMsg := fmt.Sprintf("✅ Successfully tracked **%s** with %d URLs! It will take **%d** minutes to extract the data.", listName, len(urlList), len(urlList)*3)
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: responseMsg,
			},
		})
	},
	"tracking": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		options := i.ApplicationCommandData().Options
		optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption)
		for _, opt := range options {
			optionMap[opt.Name] = opt
		}

		listName := ""
		if opt, exists := optionMap["name"]; exists {
			listName = opt.StringValue()
			reg, _ := regexp.Compile("[^a-zA-Z0-9]+")
			listName = reg.ReplaceAllString(listName, "")
			if len(listName) > 30 {
				listName = listName[:30]
			}
		}

		// Load data from helper
		trackingData, err := LoadTrackingData(listName)
		if err != nil {
			fmt.Println("Error loading tracking data:", err)
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{Content: "❌ Failed to load tracking list."},
			})
			return
		}

		var responseMsg string

		// SCENARIO A: User didn't request a specific list name (listName is empty)
		// trackingData.Urls now contains a list of all existing file names.
		if listName == "" {
			if len(trackingData.Urls) == 0 {
				responseMsg = "📂 No tracking lists found. Create one using `/track`!"
			} else {
				responseMsg = "### 📂 Available Tracking Lists:\n"
				for idx, name := range trackingData.Urls {
					if idx >= 7 { // Breaks cleanly after showing 7 items
						responseMsg += "• *and more...*\n"
						break
					}
					responseMsg += fmt.Sprintf("• **%s**\n", name)
				}
				responseMsg += "\n*Tip: Use `/tracking name:[list_name]` to see its URLs.*"
			}
		} else {
			// SCENARIO B: User provided a specific list name
			// trackingData.Urls contains the actual saved target URLs.
			responseMsg = fmt.Sprintf("### 📜 Details for **%s**:\n", listName)
			responseMsg += fmt.Sprintf("📌 **Posting Channel ID:** `%s`\n\n", trackingData.ChannelID)
			responseMsg += "**Tracked URLs:**\n"
			
			for idx, url := range trackingData.Urls {
				if idx > 6 {
					responseMsg += fmt.Sprintf("• and more .... \n") // <> prevents ugly web previews
					break
				}
				responseMsg += fmt.Sprintf("• <%s>\n", url) // <> prevents ugly web previews
			}
		}

		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: responseMsg,
			},
		})
	},
	"delete": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		// Access the options sent by the user
		options := i.ApplicationCommandData().Options

		// Convert options array to a map for easy lookup
		optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption)
		for _, opt := range options {
			optionMap[opt.Name] = opt
		}

		// 1. Safely extract the name option (defaulting to empty string if missing)
		listName := ""
		if opt, exists := optionMap["name"]; exists {
			listName = opt.StringValue()

			// Apply the same sanitization/trimming rules just in case they typed junk
			reg, _ := regexp.Compile("[^a-zA-Z0-9]+")
			listName = reg.ReplaceAllString(listName, "")
			if len(listName) > 30 {
				listName = listName[:30]
			}
		}

		_, err := DeleteTrackingData(listName)
		if err != nil {
			fmt.Println("Error deleting tracking data:", err)
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{Content: "❌ Failed to load tracking list."},
			})
			return
		}


		responseMsg := fmt.Sprintf("✅ Successfully deleted **%s**!", listName)
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: responseMsg,
			},
		})
	},
}

func main() {
	_ = godotenv.Load()
	dg, _ := discordgo.New("Bot " + os.Getenv("DISCORD_TOKEN"))

	// Route interactions to our map-picker function
	dg.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		if i.Type == discordgo.InteractionApplicationCommand {
			// Look up the command in our map and execute it if it exists
			if handler, ok := commandHandlers[i.ApplicationCommandData().Name]; ok {
				handler(s, i)
			}
		}
	})

	_ = dg.Open()
	defer dg.Close()

	// Start your 24-hour background job tracker here!
	StartScheduler(dg)

	// Register all commands in a loop
	for _, v := range commands {
		_, err := dg.ApplicationCommandCreate(dg.State.User.ID, "", v)
		if err != nil {
			fmt.Printf("Cannot create '%v' command: %v\n", v.Name, err)
		}
	}

	fmt.Println("Bot is running. Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc
}