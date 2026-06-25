package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

// DirPath is the directory where all json files are stored
const DirPath = "./links"

type UrlType []string

// Fields must be Capitalized so the json package can read/write them!
type TrackingDataType struct {
	ChannelID string  `json:"channel_id"`
	Urls      UrlType `json:"urls"`
}

// SaveTrackingData saves a specific list to its own name.json file
func SaveTrackingData(name string, channelID string, urls UrlType) error {
	// Ensure the directory exists
	if err := os.MkdirAll(DirPath, 0755); err != nil {
		return err
	}

	trackingData := TrackingDataType{
		ChannelID: channelID,
		Urls:      urls,
	}

	fileData, err := json.MarshalIndent(trackingData, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(DirPath, name+".json"), fileData, 0644)
}

// LoadTrackingData handles loading a specific file, OR returns an empty 
// struct with all saved list names inside .Urls if name is ""
func LoadTrackingData(name string) (TrackingDataType, error) {
	var trackingData TrackingDataType

	// SCENARIO A: Fetch all list names from the directory
	if len(name) == 0 {
		files, err := os.ReadDir(DirPath)
		if err != nil {
			// If directory doesn't exist yet, return gracefully with empty data
			if os.IsNotExist(err) {
				return trackingData, nil
			}
			return trackingData, err
		}

		var listNames UrlType
		for _, entry := range files {
			if !entry.IsDir() && filepath.Ext(entry.Name()) == ".json" {
				filename := entry.Name()
				// Strip out ".json" suffix to get the raw list name
				cleanName := filename[:len(filename)-5]
				listNames = append(listNames, cleanName)
			}
		}

		trackingData.ChannelID = ""
		trackingData.Urls = listNames
		return trackingData, nil
	}

	// SCENARIO B: Load a specific tracking file
	filePath := filepath.Join(DirPath, name+".json")
	fileData, err := os.ReadFile(filePath)
	if err != nil {
		return trackingData, err
	}

	err = json.Unmarshal(fileData, &trackingData)
	if err != nil {
		return trackingData, err
	}

	return trackingData, nil
}

// DeleteTrackingData deletes the specific name.json file
func DeleteTrackingData(name string) (bool, error) {
	filePath := filepath.Join(DirPath, name+".json")
	err := os.Remove(filePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil 
		}
		return false, err
	}
	return true, nil
}