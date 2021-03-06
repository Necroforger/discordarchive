package discordarchive

import (
	"database/sql"
	"errors"

	"github.com/bwmarrin/discordgo"
)

// Count ...
func Count(db *sql.DB, query string, args ...interface{}) (int, error) {
	var count int
	row := db.QueryRow(query, args...)
	err := row.Scan(&count)
	if err != nil {
		return -1, err
	}
	return count, nil
}

// ChannelMessages ...
func ChannelMessages(db *sql.DB, channelID string, offset, limit int) ([]*discordgo.Message, error) {
	var rows *sql.Rows
	if limit > 0 || offset > 0 {
		r, err := db.Query("SELECT * FROM messages WHERE channelid=? ORDER BY messageID LIMIT ? OFFSET ?", channelID, limit, offset)
		if err != nil {
			return nil, err
		}
		rows = r
	} else {
		r, err := db.Query("SELECT * FROM messages WHERE channelid=? ORDER BY messageID", channelID)
		if err != nil {
			return nil, err
		}
		rows = r
	}
	messages, err := ScanMessages(rows)
	if err != nil {
		return nil, err
	}
	return messages, nil
}

// Channel ...
func Channel(db *sql.DB, channelID string) (*discordgo.Channel, error) {
	rows, err := db.Query("SELECT * FROM channels WHERE channelID=?", channelID)
	if err != nil {
		return nil, err
	}

	if !rows.Next() {
		return nil, errors.New("No result found")
	}
	channel, err := scanChannel(rows)
	if err != nil {
		return nil, err
	}

	return channel, nil
}

// Guild ...
func Guild(db *sql.DB, guildID string) (*discordgo.Guild, error) {
	rows, err := db.Query("SELECT * FROM guilds WHERE guildID=?", guildID)
	if err != nil {
		return nil, err
	}

	guilds, err := ScanGuilds(rows)
	if err != nil {
		return nil, err
	}

	if len(guilds) == 0 {
		return nil, errors.New("guild not found: " + guildID)
	}

	return guilds[0], nil
}

// Guilds ...
func Guilds(db *sql.DB) ([]*discordgo.Guild, error) {
	rows, err := db.Query("SELECT * FROM guilds")
	if err != nil {
		return nil, err
	}

	guilds, err := ScanGuilds(rows)
	if err != nil {
		return nil, err
	}

	return guilds, nil
}

// Channels ...
func Channels(db *sql.DB, guildID string) ([]*discordgo.Channel, error) {
	rows, err := db.Query("SELECT * FROM channels WHERE guildID=?", guildID)
	if err != nil {
		return nil, err
	}

	channels, err := ScanChannels(rows)
	if err != nil {
		return nil, err
	}

	return channels, nil
}
