// Package metadata carries server-computed metadata about ActivityStream documents -- knowledge
// that is ABOUT a document but never part of its wire value. It holds two layers: document facts
// (category, relationships, response counts), which are the same for every viewer and are persisted
// alongside cached documents; and per-load values (moderation Labels, the NoStore policy), which are
// attached at load time and never persisted or serialized.
package metadata

import "github.com/benpate/hannibal/vocab"

// Metadata contains server-computed metadata about a single document: persisted document facts,
// plus per-load values that are never persisted or serialized.
type Metadata struct {

	// Document facts: identical for every viewer, persisted with the cached document.

	HashedID         string `bson:"hashedId,omitempty"`         // HashedID is a unique identifier for this document, used to prevent duplicate records
	DocumentCategory string `bson:"documentCategory,omitempty"` // High-level category of the document [Activity, Actor, Object, Collection]
	RelationType     string `bson:"relationType,omitempty"`     // If this document is related to another document, this contains the type of relation [Reply, Announce, Like, Dislike]
	RelationHref     string `bson:"relationHref,omitempty"`     // If this document is related to another document, this contains the URL of the related document
	Replies          int64  `bson:"replies,omitempty"`          // Replies is the number of replies to this document
	Announces        int64  `bson:"announces,omitempty"`        // Announces is the number of times this document has been announced / reposted
	Likes            int64  `bson:"likes,omitempty"`            // Likes is the number of times this document has been liked

	// Per-load values: set only by server code, never persisted or serialized.

	Labels  LabelSet `bson:"-" json:"-"` // Labels is the current viewer's moderation verdict for this document
	NoStore bool     `bson:"-" json:"-"` // NoStore prevents every cache from writing this document
}

// New returns a fully initialized Metadata object.
func New() Metadata {
	return Metadata{}
}

// Clone returns a copy of this Metadata that shares no mutable state with the original.
func (metadata Metadata) Clone() Metadata {
	result := metadata
	result.Labels = metadata.Labels.Clone()
	return result
}

// IsRuleHidden returns TRUE if the current viewer's rules hide this document (a block or a mute).
func (metadata Metadata) IsRuleHidden() bool {
	return metadata.Labels.IsHidden()
}

// HasReplies returns TRUE if this document has one or more Replies
func (metadata Metadata) HasReplies() bool {
	return metadata.Replies > 0
}

// HasAnnounces returns TRUE if this document has one or more Announces
func (metadata Metadata) HasAnnounces() bool {
	return metadata.Announces > 0
}

// HasLikes returns TRUE if this document has one or more Likes
func (metadata Metadata) HasLikes() bool {
	return metadata.Likes > 0
}

// HasRelationship returns TRUE if this document has a relationship
func (metadata Metadata) HasRelationship() bool {
	if metadata.RelationType == "" {
		return false
	}

	if metadata.RelationHref == "" {
		return false
	}

	return true
}

// SetRelationCount updates the designated relation with a new count,
// returning TRUE if the value has been changed.
func (metadata *Metadata) SetRelationCount(relationType string, count int64) bool {

	switch relationType {

	case vocab.RelationTypeReply:
		if metadata.Replies != count {
			metadata.Replies = count
			return true
		}

	case vocab.RelationTypeAnnounce:
		if metadata.Announces != count {
			metadata.Announces = count
			return true
		}

	case vocab.RelationTypeLike:
		if metadata.Likes != count {
			metadata.Likes = count
			return true
		}
	}

	return false
}
