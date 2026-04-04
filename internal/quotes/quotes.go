package quotes

import (
	"bufio"
	"math/rand"
	"os"
	"strings"
	"time"
)

var builtinQuotes = []string{
	"Do one thing every day that scares you.",
	"Talk is cheap. Show me the code.",
	"Done is better than perfect.",
	"First, solve the problem. Then, write the code.",
	"Simplicity is the ultimate sophistication.",
	"The only way to do great work is to love what you do.",
	"Stay hungry, stay foolish.",
	"Make it work, make it right, make it fast.",
	"Code is like humor. When you have to explain it, it's bad.",
	"It always seems impossible until it's done.",
	"A clean desk is a sign of a cluttered desk drawer.",
	"We are what we repeatedly do. Excellence, then, is not an act, but a habit.",
	"Your future is created by what you do today, not tomorrow.",
	"Debugging is twice as hard as writing the code in the first place.",
	"Don't watch the clock; do what it does. Keep going.",
	"The best way to predict the future is to invent it.",
	"Perfection is achieved not when there is nothing more to add, but when there is nothing left to take away.",
	"Creativity is intelligence having fun.",
	"Focus on being productive instead of busy.",
	"Small deeds done are better than great deeds planned.",
	"What we think, we become.",
	"Happiness depends upon ourselves.",
	"An unexamined life is not worth living.",
	"He who has a why to live can bear almost any how.",
}

// expandHome replaces a leading "~" with the user's home directory.
func expandHome(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			return strings.Replace(path, "~", home, 1)
		}
	}
	return path
}

// readCustomQuotes reads non-empty lines from the given file.
func readCustomQuotes(path string) []string {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	var quotes []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			quotes = append(quotes, line)
		}
	}
	return quotes
}

// Get returns the quote for today.
// If source is "custom" and a valid custom file is provided, it uses that;
// otherwise it falls back to the built-in library.
// The selection is seeded with today's date so the same day always returns the same quote.
func Get(source, customFile string) string {
	var pool []string

	if source == "custom" && customFile != "" {
		pool = readCustomQuotes(expandHome(customFile))
	}

	if len(pool) == 0 {
		pool = builtinQuotes
	}

	seed := time.Now().Format("20060102")
	r := rand.New(rand.NewSource(int64(hash(seed))))
	idx := r.Intn(len(pool))
	return pool[idx]
}

// hash converts a string to an int64 seed.
func hash(s string) int64 {
	var h int64
	for i := 0; i < len(s); i++ {
		h = h*31 + int64(s[i])
	}
	return h
}
