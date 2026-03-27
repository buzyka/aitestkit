package semantic

import "encoding/json"

var checkResultSchema = json.RawMessage(`{
	"type": "object",
	"properties": {
		"score": {
			"type": "integer",
			"description": "Score from 1 to 10"
		},
		"description": {
			"type": "string",
			"description": "Short explanation of the score"
		}
	},
	"required": ["score", "description"],
	"additionalProperties": false
}`)
