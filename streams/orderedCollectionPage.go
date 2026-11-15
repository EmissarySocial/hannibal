package streams

import (
	"encoding/json"

	"github.com/benpate/derp"
	"github.com/benpate/hannibal/vocab"
	"github.com/benpate/rosetta/mapof"
)

// OrderedCollectionPage is used to represent ordered subsets of items from an OrderedCollection. Refer to the Activity Streams 2.0 Core for a complete description of the OrderedCollectionPage object.
// https://www.w3.org/ns/activitystreams#OrderedCollectionPage
type OrderedCollectionPage struct {
	Context      Context `json:"@context,omitempty"     bson:"context,omitempty"`
	Type         string  `json:"type,omitempty"         bson:"type,omitempty"`
	ID           string  `json:"id,omitempty"           bson:"id,omitempty"`           // Provides the globally unique identifier for an Object or Link.
	Summary      string  `json:"summary,omitempty"      bson:"summary,omitempty"`      // A natural language summarization of the object encoded as HTML. Multiple language tagged summaries may be provided.
	TotalItems   int     `json:"totalItems,omitempty"   bson:"totalItems,omitempty"`   // A non-negative integer specifying the total number of objects contained by the logical view of the collection. This number might not reflect the actual number of items serialized within the Collection object instance.
	Current      string  `json:"current,omitempty"      bson:"current,omitempty"`      // In a paged Collection, indicates the page that contains the most recently updated member items.
	First        string  `json:"first,omitempty"        bson:"first,omitempty"`        // In a paged Collection, indicates the furthest preceding page of items in the collection.
	Last         string  `json:"last,omitempty"         bson:"last,omitempty"`         // In a paged Collection, indicates the furthest proceeding page of the collection.
	StartIndex   int     `json:"startIndex,omitempty"   bson:"startIndex,omitempty"`   // A non-negative integer value identifying the relative position within the logical view of a strictly ordered collection.
	PartOf       string  `json:"partOf,omitempty"       bson:"partOf,omitempty"`       // dentifies the Collection to which a CollectionPage objects items belong.
	Prev         string  `json:"prev,omitempty"         bson:"prev,omitempty"`         // In a paged Collection, identifies the previous page of items.
	Next         string  `json:"next,omitempty"         bson:"next,omitempty"`         // In a paged Collection, indicates the next page of items.
	OrderedItems []any   `json:"orderedItems,omitempty" bson:"orderedItems,omitempty"` // Identifies the items contained in a collection. The items might be ordered or unordered.
}

// NewOrderedCollectionPage returns a new OrderedCollectionPage with the provided ID and parent.
func NewOrderedCollectionPage(pageID string, partOf string) OrderedCollectionPage {
	return OrderedCollectionPage{
		Context:      DefaultContext(),
		Type:         vocab.CoreTypeOrderedCollectionPage,
		ID:           pageID,
		PartOf:       partOf,
		OrderedItems: make([]any, 0),
	}
}

// UnmarshalJSON populates the OrderedCollectionPage from its JSON representation.
func (c *OrderedCollectionPage) UnmarshalJSON(data []byte) error {

	const location = "hannibal.streams.OrderedCollectionPage.UnmarshalJSON"

	result := mapof.NewAny()

	if err := json.Unmarshal(data, &result); err != nil {
		return derp.Wrap(err, location, "Error unmarshalling JSON", string(data))
	}

	return c.UnmarshalMap(result)
}

// UnmarshalMap populates the OrderedCollectionPage from a generic map representation.
func (c *OrderedCollectionPage) UnmarshalMap(data mapof.Any) error {

	const location = "hannibal.streams.OrderedCollectionPage.UnmarshalMap"

	if dataType := data.GetString("type"); dataType != vocab.CoreTypeOrderedCollectionPage {
		return derp.Internal(location, "Invalid type", dataType)
	}

	c.Type = vocab.CoreTypeOrderedCollectionPage
	c.ID = data.GetString("id")
	c.Summary = data.GetString("summary")
	c.TotalItems = data.GetInt("totalItems")
	c.Current = data.GetString("current")
	c.First = data.GetString("first")
	c.Last = data.GetString("last")
	c.StartIndex = data.GetInt("startIndex")
	c.PartOf = data.GetString("partOf")
	c.Prev = data.GetString("prev")
	c.Next = data.GetString("next")

	// An ordered collection lists its members in `orderedItems`. Some servers write `items`,
	// which is read only when the correct property is missing.
	dataItems, found := data[vocab.PropertyOrderedItems]

	if !found {
		dataItems, found = data[vocab.PropertyItems]
	}

	if found {
		if items, ok := UnmarshalItems(dataItems); ok {
			c.OrderedItems = items
		}
	}

	return nil
}
