package main

import (
    "encoding/json"
    "fmt"
    "io/ioutil"
    "os"
    "strings"

    "github.com/bwmarrin/discordgo"
    "github.com/joho/godotenv"
)

type Verse struct {
    BookName string `json:"book_name"`
    Book     int    `json:"book"`
    Chapter  int    `json:"chapter"`
    Verse    int    `json:"verse"`
    Text     string `json:"text"`
}

type BibleData struct {
    Metadata struct {
        Name string `json:"name"`
    } `json:"metadata"`
    Verses []Verse `json:"verses"`
}

type BibleBook struct {
    Name   string
    Verses []Verse
}

type Config struct {
    Token string
}

func loadConfig() Config {
    if err := godotenv.Load(); err != nil {
        fmt.Println("Error loading .env file")
        os.Exit(1)
    }

    token := os.Getenv("DISCORD_BOT_TOKEN")
    if token == "" {
        fmt.Println("DISCORD_BOT_TOKEN environment variable not set")
        os.Exit(1)
    }
    return Config{Token: token}
}

func loadBibleData() ([]BibleBook, error) {
    file, err := ioutil.ReadFile("net.json")
    if err != nil {
        return nil, fmt.Errorf("failed to read net.json: %v", err)
    }

    var bibleData BibleData
    if err := json.Unmarshal(file, &bibleData); err != nil {
        return nil, fmt.Errorf("failed to parse net.json: %v", err)
    }

    bookMap := make(map[string][]Verse)
    for _, verse := range bibleData.Verses {
        bookMap[verse.BookName] = append(bookMap[verse.BookName], verse)
    }

    var books []BibleBook
    for bookName, verses := range bookMap {
        books = append(books, BibleBook{
            Name:   bookName,
            Verses: verses,
        })
    }

    for i := 0; i < len(books)-1; i++ {
        for j := i + 1; j < len(books); j++ {
            if books[i].Verses[0].Book > books[j].Verses[0].Book {
                books[i], books[j] = books[j], books[i]
            }
        }
    }

    return books, nil
}

func splitMessage(text string) []string {
    const maxLength = 1000
    var messages []string
    for len(text) > 0 {
        if len(text) <= maxLength {
            messages = append(messages, text)
            break
        }

        // Find the last newline before maxLength
        lastNewline := strings.LastIndex(text[:maxLength], "\n")
        if lastNewline == -1 {
            lastNewline = maxLength
        }
        messages = append(messages, text[:lastNewline])
        text = text[lastNewline:]
    }
    return messages
}

func formatBook(book BibleBook) string {
    var builder strings.Builder

    for _, verse := range book.Verses {
        builder.WriteString(fmt.Sprintf("%d:%d %s\n", verse.Chapter, verse.Verse, verse.Text))
    }

    return builder.String()
}

func setupServer(s *discordgo.Session, guildID string, books []BibleBook) error {
    guild, err := s.Guild(guildID)
    if err != nil {
        return fmt.Errorf("failed to get guild: %v", err)
    }

    var everyoneRoleID string
    for _, role := range guild.Roles {
        if role.Name == "@everyone" {
            everyoneRoleID = role.ID
            break
        }
    }

    if everyoneRoleID == "" {
        return fmt.Errorf("could not find @everyone role")
    }

    for _, book := range books {
        channelName := strings.ToLower(strings.ReplaceAll(book.Name, " ", "-"))
        if len(channelName) > 100 {
            channelName = channelName[:100]
        }

        channel, err := s.GuildChannelCreate(guildID, channelName, discordgo.ChannelTypeGuildText)
        if err != nil {
            fmt.Printf("Warning: failed to create channel for %s: %v\n", book.Name, err)
            continue
        }

        err = s.ChannelPermissionSet(channel.ID, everyoneRoleID, discordgo.PermissionOverwriteTypeRole,
            discordgo.PermissionReadMessageHistory|discordgo.PermissionViewChannel,
            discordgo.PermissionSendMessages|discordgo.PermissionManageMessages)
        if err != nil {
            fmt.Printf("Warning: failed to set permissions for %s: %v\n", book.Name, err)
        }

        bookText := formatBook(book)
        messages := splitMessage(bookText)
        for _, msg := range messages {
            _, err := s.ChannelMessageSend(channel.ID, msg)
            if err != nil {
                fmt.Printf("Warning: failed to send message to %s: %v\n", book.Name, err)
            }
        }
    }

    return nil
}

func onReady(s *discordgo.Session, event *discordgo.Ready) {
    fmt.Println("Bot is ready!")
}

func onGuildCreate(s *discordgo.Session, event *discordgo.GuildCreate) {
    fmt.Printf("Joined guild: %s (%s)\n", event.Guild.Name, event.Guild.ID)

    books, err := loadBibleData()
    if err != nil {
        fmt.Printf("Error loading Bible data: %v\n", err)
        return
    }

    err = setupServer(s, event.Guild.ID, books)
    if err != nil {
        fmt.Printf("Error setting up server: %v\n", err)
    } else {
        fmt.Println("Server setup completed successfully")
    }
}

func main() {
    config := loadConfig()

    dg, err := discordgo.New("Bot " + config.Token)
    if err != nil {
        fmt.Printf("Error creating Discord session: %v\n", err)
        return
    }

    dg.AddHandler(onReady)
    dg.AddHandler(onGuildCreate)

    err = dg.Open()
    if err != nil {
        fmt.Printf("Error opening connection: %v\n", err)
        return
    }

    fmt.Println("Bot is running. Press CTRL+C to exit.")

    select {}
}
