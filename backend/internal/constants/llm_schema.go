package constants

// LLM response schema for structured query generation
const LLMResponseSchema = `{
    "type": "object",
    "required": [
      "assistantMessage"
    ],
    "properties": {
      "queries": {
        "type": "array",
        "items": {
          "type": "object",
          "required": [
            "query",
            "queryType",
            "explanation",
            "isCritical",
            "canRollback",
            "estimateResponseTime"
          ],
          "properties": {
            "query": {
              "type": "string",
              "description": "SQL query to fetch order details."
            },
            "tables": {
              "type": "string",
              "description": "Tables being used in the query(comma separated)"
            },
            "queryType": {
              "type": "string",
              "description": "SQL query type(SELECT,UPDATE,INSERT,DELETE,DDL)"
            },
            "isCritical": {
              "type": "boolean",
              "description": "Indicates if the query is critical."
            },
            "canRollback": {
              "type": "boolean",
              "description": "Indicates if the operation can be rolled back."
            },
            "explanation": {
              "type": "string",
              "description": "Description of what the query does."
            },
            "exampleResult": {
              "type": "array",
              "items": {
                "type": "object",
                "description": "Key-value pairs representing column names and example values.",
                "additionalProperties": {
                  "type": "string"
                }
              },
              "description": "An example array of results that the query might return."
            },
            "rollbackQuery": {
              "type": "string",
              "description": "Query to undo this operation (if canRollback=true), default empty"
            },
            "estimateResponseTime": {
              "type": "number",
              "description": "Estimated time (in milliseconds) to fetch the response."
            },
            "rollbackDependentQuery": {
              "type": "string",
              "description": "Query to run by the user to get the required data that AI needs in order to write a successful rollbackQuery"
            }
          },
          "additionalProperties": false
        },
        "description": "List of queries related to orders."
      },
      "assistantMessage": {
        "type": "string",
        "description": "Message from the assistant providing context about the orders."
      }
    },
    "additionalProperties": false
  }`

// LLMResponse represents the structured response from LLM
type LLMResponse struct {
	Queries          []QueryInfo `json:"queries,omitempty"`
	AssistantMessage string      `json:"assistantMessage"`
}

// QueryInfo represents a single query in the LLM response
type QueryInfo struct {
	Query                  string              `json:"query"`
	Tables                 string              `json:"tables,omitempty"`
	QueryType              string              `json:"queryType"`
	IsCritical             bool                `json:"isCritical"`
	CanRollback            bool                `json:"canRollback"`
	Explanation            string              `json:"explanation"`
	ExampleResult          []map[string]string `json:"exampleResult,omitempty"`
	RollbackQuery          string              `json:"rollbackQuery,omitempty"`
	EstimateResponseTime   float64             `json:"estimateResponseTime"`
	RollbackDependentQuery string              `json:"rollbackDependentQuery,omitempty"`
}
