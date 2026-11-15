package corpus

import (
	"embed"
	"encoding/json"
	"path"
	"slices"
	"strings"

	"github.com/benpate/derp"
)

// files holds every example document, embedded so that a test in another module can read them
//
//go:embed fediverse/*.json w3c/*.json rejected/*.json
var files embed.FS

// Example is one ActivityStreams document, with what a delivery test should expect of it
type Example struct {
	Name       string   // Name of the file it came from, without the extension
	Source     string   // The software, specification or situation that the document is shaped like
	JSON       []byte   // The document itself
	Recipients []string // Every address it should be delivered to, in order. Nil when no answer is recorded.
}

// World names the local actor that sends the fediverse examples, and who follows it
type World struct {
	Actor     string   // URL of the sending actor
	Followers string   // URL of that actor's followers collection
	Members   []string // The members of the followers collection, as actor URLs and mailto: URLs
}

// fediverseFile is the layout of a file in the fediverse folder
type fediverseFile struct {
	Source     string          `json:"source"`
	Recipients []string        `json:"recipients"`
	Activity   json.RawMessage `json:"activity"`
}

// Fediverse returns the examples shaped like what fediverse servers publish, rewritten so that
// the sending actor and its followers collection are the ones in the given World
func Fediverse(world World) ([]Example, error) {

	const location = "hannibal.corpus.Fediverse"

	replacer := strings.NewReplacer(tokenActor, world.Actor, tokenFollowers, world.Followers)

	names, err := exampleNames(folderFediverse)

	if err != nil {
		return nil, derp.Wrap(err, location, "Listing examples")
	}

	result := make([]Example, 0, len(names))

	for _, name := range names {

		contents, err := files.ReadFile(path.Join(folderFediverse, name+extension))

		if err != nil {
			return nil, derp.Wrap(err, location, "Reading example", name)
		}

		file := fediverseFile{}

		if err := json.Unmarshal(contents, &file); err != nil {
			return nil, derp.Wrap(err, location, "Parsing example", name)
		}

		result = append(result, Example{
			Name:       name,
			Source:     file.Source,
			JSON:       []byte(replacer.Replace(string(file.Activity))),
			Recipients: expandRecipients(file.Recipients, world),
		})
	}

	return result, nil
}

// W3C returns the examples that the W3C publishes with the ActivityStreams specifications.
// They record no recipients, and most of them are objects and collections, not activities.
func W3C() ([]Example, error) {
	return verbatim(folderW3C, "W3C ActivityStreams 2.0 test documents")
}

// Rejected returns the W3C documents that a strict ActivityStreams parser refuses, each of
// which still parses as a JSON object
func Rejected() ([]Example, error) {
	return verbatim(folderRejected, "W3C ActivityStreams 2.0 test documents that a parser should refuse")
}

// Document returns the example decoded as plain JSON. Every call returns a new value that
// shares nothing with an earlier one.
func (example Example) Document() (map[string]any, error) {

	const location = "hannibal.corpus.Example.Document"

	result := make(map[string]any)

	if err := json.Unmarshal(example.JSON, &result); err != nil {
		return nil, derp.Wrap(err, location, "Parsing example", example.Name)
	}

	return result, nil
}

// verbatim returns every document in a folder, as it was published
func verbatim(folder string, source string) ([]Example, error) {

	const location = "hannibal.corpus.verbatim"

	names, err := exampleNames(folder)

	if err != nil {
		return nil, derp.Wrap(err, location, "Listing examples")
	}

	result := make([]Example, 0, len(names))

	for _, name := range names {

		contents, err := files.ReadFile(path.Join(folder, name+extension))

		if err != nil {
			return nil, derp.Wrap(err, location, "Reading example", name)
		}

		result = append(result, Example{Name: name, Source: source, JSON: contents})
	}

	return result, nil
}

// exampleNames returns the name of every example in a folder, sorted, without the extension
func exampleNames(folder string) ([]string, error) {

	entries, err := files.ReadDir(folder)

	if err != nil {
		return nil, derp.Wrap(err, "hannibal.corpus.exampleNames", "Reading folder", folder)
	}

	result := make([]string, 0, len(entries))

	for _, entry := range entries {
		if name, isExample := strings.CutSuffix(entry.Name(), extension); isExample {
			result = append(result, name)
		}
	}

	slices.Sort(result)
	return result, nil
}

// expandRecipients rewrites a recorded list of recipients for one World: the actor and the
// followers collection take their real URLs, and the collection is replaced by its members.
// An address keeps the place where it first appears.
func expandRecipients(recipients []string, world World) []string {

	result := make([]string, 0, len(recipients)+len(world.Members))

	for _, recipient := range recipients {

		switch recipient {

		case tokenFollowers:
			result = appendNew(result, world.Members...)

		case tokenActor:
			result = appendNew(result, world.Actor)

		default:
			result = appendNew(result, recipient)
		}
	}

	return result
}

// appendNew appends each value that the slice does not hold yet
func appendNew(values []string, additions ...string) []string {

	for _, addition := range additions {
		if !slices.Contains(values, addition) {
			values = append(values, addition)
		}
	}

	return values
}
