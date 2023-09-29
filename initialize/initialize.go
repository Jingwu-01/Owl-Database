// A package containing a method to initialize OwlDB from the commandline.
package initialize

import (
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
func Initialize() (int, *jsonschema.Schema, string, bool, error) {
	// Initialize flags
	portFlag := flag.Int("p", 3318, "Port number")
	schemaFlag := flag.String("s", "", "Schema file name")
	tokenFlag := flag.String("t", "", "Token file name")
	loggerFlag := flag.Int("l", 0, "Logger output level, -1 for debug, 1 for only errors")
	testFlag := flag.Bool("i", false, "true to initialize a default database for testing")
	flag.Parse()

	// Ensure we got a schema file
	if *schemaFlag == "" {
		slog.Error("Missing schema", "error", errors.New("Missing schema"))
		return 0, nil, "", false, errors.New("Missing Schema")
	}

	// Compile the schema
	schema, err := jsonschema.Compile(*schemaFlag)

	// Check for errors.
	if err != nil {
		slog.Error("Invalid schema", "error", err)
		return 0, nil, "", false, errors.New("Invalid schema")
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

	return *portFlag, schema, *tokenFlag, *testFlag, nil
}
