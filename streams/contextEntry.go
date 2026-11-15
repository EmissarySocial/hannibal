package streams

import (
	"encoding/json"

	"github.com/benpate/derp"
)

// ContextEntry represents a single entry in a JSON-LD @context: a vocabulary,
// an optional language, and an optional map of namespace extensions.
// https://www.w3.org/TR/json-ld/#the-context
type ContextEntry struct {
	Vocabulary string            // The primary vocabulary represented by the context/document.
	Language   string            // The language
	Extensions map[string]string // a map of additional namespaces that are included in this context/document.
}

// NewContextEntry returns a new ContextEntry for the provided vocabulary.
func NewContextEntry(vocabulary string) ContextEntry {
	return ContextEntry{
		Vocabulary: vocabulary,
		Language:   "und",
		Extensions: make(map[string]string),
	}
}

// WithLanguage sets the entry's language and returns the entry for chaining.
func (entry *ContextEntry) WithLanguage(language string) *ContextEntry {
	entry.Language = language
	return entry
}

// WithExtension adds a namespace extension to the entry and returns the entry for chaining.
func (entry *ContextEntry) WithExtension(key string, value string) *ContextEntry {
	if len(entry.Extensions) == 0 {
		entry.Extensions = make(map[string]string)
	}

	entry.Extensions[key] = value
	return entry
}

// MarshalJSON encodes the entry as a JSON string (vocabulary only) or object (with language/extensions).
func (entry ContextEntry) MarshalJSON() ([]byte, error) {

	// If this context only has a vocabulary, then
	// use the short-form "string only" syntax
	if entry.IsVocabularyOnly() {
		return json.Marshal(entry.Vocabulary)
	}

	// Otherwise, use the long-form syntax as a JSON object
	result := make(map[string]any)
	result["@vocab"] = entry.Vocabulary

	if entry.IsLanguageDefined() {
		result["@language"] = entry.Language
	}

	if entry.HasExtensions() {
		for key, value := range entry.Extensions {
			result[key] = value
		}
	}

	return json.Marshal(result)
}

// IsVocabularyOnly returns TRUE if the entry defines only a vocabulary (no language or extensions).
func (entry ContextEntry) IsVocabularyOnly() bool {
	if entry.IsLanguageDefined() {
		return false
	}

	if entry.HasExtensions() {
		return false
	}

	return true
}

// IsLanguageDefined returns TRUE if the entry defines a language other than the undefined default.
func (entry ContextEntry) IsLanguageDefined() bool {
	if entry.Language == "" {
		return false
	}

	if entry.Language == "und" {
		return false
	}

	return true
}

// HasExtensions returns TRUE if the entry defines one or more namespace extensions.
func (entry ContextEntry) HasExtensions() bool {
	return len(entry.Extensions) > 0
}

// UnmarshalJSON decodes the entry from a JSON string (vocabulary only) or object. A term whose
// definition is not a string cannot be held in Extensions, and is skipped.
func (entry *ContextEntry) UnmarshalJSON(data []byte) error {

	const location = "hannibal.streams.ContextEntry.UnmarshalJSON"

	// The short form is the vocabulary alone
	var vocabulary string

	if err := json.Unmarshal(data, &vocabulary); err == nil {
		*entry = NewContextEntry(vocabulary)
		return nil
	}

	// The long form is an object of `@vocab`, `@language`, and named terms
	var terms map[string]any

	if err := json.Unmarshal(data, &terms); err != nil {
		return derp.Wrap(err, location, "Context entry must be a string or an object")
	}

	result := NewContextEntry("")

	for name, value := range terms {

		text, isText := value.(string)

		if !isText {
			continue
		}

		switch name {

		case "@vocab":
			result.Vocabulary = text

		case "@language":
			result.Language = text

		default:
			result.Extensions[name] = text
		}
	}

	*entry = result
	return nil
}
