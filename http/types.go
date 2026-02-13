package http

import "jabberwocky238/jw238dns/types"

// AddRecordRequest is the request body for POST /dns/add.
type AddRecordRequest struct {
	Domain string          `json:"domain" binding:"required"`
	Type   types.RecordType `json:"type" binding:"required"`
	Value  []string        `json:"value" binding:"required,min=1"`
	TTL    uint32          `json:"ttl"`
}

// DeleteRecordRequest is the request body for POST /dns/delete.
type DeleteRecordRequest struct {
	Domain string          `json:"domain" binding:"required"`
	Type   types.RecordType `json:"type" binding:"required"`
}

// UpdateRecordRequest is the request body for POST /dns/update.
type UpdateRecordRequest struct {
	Domain string          `json:"domain" binding:"required"`
	Type   types.RecordType `json:"type" binding:"required"`
	Value  []string        `json:"value" binding:"required,min=1"`
	TTL    uint32          `json:"ttl"`
}
