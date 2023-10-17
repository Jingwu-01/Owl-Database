// A package containing a method to initialize OwlDB from the commandline.
package initialize

import (
	"encoding/json"
	"errors"
	"flag"
	"log/slog"
	"os"

	"github.com/santhosh-tekuri/jsonschema/v5"
)

// Initialize sets up flags for inputs and compiles
// the input schema file into a jsonschema.Schema object.
// Returns the port number, schema object, and the token
// file's path.
func Initialize() (int, *jsonschema.Schema, map[string]string, error) {
	// Initialize flags
	portFlag := flag.Int("p", 3318, "Port number")
	schemaFlag := flag.String("s", "", "Schema file name")
	tokenFlag := flag.String("t", "", "Token file name")
	loggerFlag := flag.Int("l", 0, "Logger output level, -1 for debug, 1 for only errors")
	flag.Parse()

	var tokenmap map[string]string

	// Ensure we got a schema file
	if *schemaFlag == "" {
		slog.Error("Missing schema", "error", errors.New("missing schema"))
		return 0, nil, tokenmap, errors.New("missing schema")
	}

	// Compile the schema
	schema, err := jsonschema.Compile(*schemaFlag)

	// Check for errors.
	if err != nil {
		slog.Error("Invalid schema", "error", err)
		return 0, nil, tokenmap, errors.New("invalid schema")
	}

	// If the user inputs a token file.
	if *tokenFlag != "" {
		// Read in token file.
		tokens, err := os.ReadFile(*tokenFlag)
		if err != nil {
			slog.Error("Error reading token file", "error", err)
			return 0, nil, tokenmap, errors.New("token file error")
		}

		// Unmarshal it.
		err = json.Unmarshal(tokens, &tokenmap)
		if err != nil {
			slog.Error("Error marshalling token file", "error", err)
			return 0, nil, tokenmap, errors.New("marshalling tokens error")
		}
	}

	// Set to debug and above
	if *loggerFlag == -1 {
		h := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug})
		slog.SetDefault(slog.New(h))
	}

	// Set to error only
	if *loggerFlag == 1 {
		h := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})
		slog.SetDefault(slog.New(h))
	}

	return *portFlag, schema, tokenmap, nil
}
