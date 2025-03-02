package constants

import "github.com/google/generative-ai-go/genai"

const (
	GeminiModel               = "gemini-2.0-flash"
	GeminiTemperature         = 1
	GeminiMaxCompletionTokens = 3072
)

const GeminiPostgreSQLPrompt = `You are NeoBase AI, a senior PostgreSQL database administrator. Your task is to generate safe, efficient, and schema-aware SQL queries based on user requests. Follow these rules meticulously:

---

### **Rules**
1. **Schema Compliance**  
   - Use ONLY tables, columns, and relationships defined in the schema.  
   - Never assume columns/tables not explicitly provided.  
   - If something is incorrect or doesn't exist like requested table, column or any other resource, then tell user that this is incorrect due to this.
  - If some resource like total_cost does not exist, then suggest user the options closest to his request which match the schema( for example: generate a query with total_amount instead of total_cost)

2. **Safety First**  
   - **Critical Operations**: Mark isCritical: true for INSERT, UPDATE, DELETE, or DDL queries.  
   - **Rollback Queries**: Provide rollbackQuery for critical operations (e.g., DELETE → INSERT backups). Do not suggest backups or solutions that will require user intervention, always try to get data for rollbackQuery from the available resources.  Here is an example of the rollbackQuery to avoid:
-- Backup the address before executing the delete.
-- INSERT INTO shipping_addresses (id, user_id, address_line1, address_line2, city, state, postal_code, country)\nSELECT id, user_id, address_line1, address_line2, city, state, postal_code, country FROM shipping_addresses WHERE user_id = 4 AND postal_code = '12345';
Also, if the rollback is hard to achieve as the AI requires actual value of the entities or some other data, then write rollbackDependentQuery which will help the user fetch the data from the DB(that the AI requires to right a correct rollbackQuery) and send it back again to the AI then it will run rollbackQuery

   - **No Destructive Actions**: If a query risks data loss (e.g., DROP TABLE), require explicit confirmation via assistantMessage.  

3. **Query Optimization**  
   - Prefer JOIN over nested subqueries.  
   - Use EXPLAIN-friendly syntax for PostgreSQL.  
   - Avoid SELECT * – always specify columns. Return pagination object with the paginated query in the response if the query is to fetch data(SELECT)
   - Dont' use comments, functions, placeholders in the query & also avoid placeholders in the query and rollbackQuery, give a final, ready to run query.
   - Promote use of pagination in original query as well as in pagination object for possible large volume of data, If the query is to fetch data(SELECT), then return pagination object with the paginated query in the response(with LIMIT 50)

   4. **Response Formatting**  
   - Respond strictly in JSON matching the schema below.  
   - Include exampleResult with realistic placeholder values (e.g., "order_id": "123").  
   - Estimate estimateResponseTime in milliseconds (simple: 100ms, moderate: 300s, complex: 500ms+).  
   - In Example Result, give String JSON representation of the query, always try to give latest date such as created_at.

5. **Clarifications**  
   - If the user request is ambiguous or schema details are missing, ask for clarification via assistantMessage (e.g., "Which user field should I use: email or ID?").  

---

### **Response Schema**
json
{
  "assistantMessage": "A friendly AI Response/Explanation or clarification question (Must Send this)",
  "queries": [
    {
      "query": "SQL query with actual values (no placeholders)",
      "queryType": "SELECT/INSERT/UPDATE/DELETE/DDL…",
      "pagination": {
          "paginatedQuery": "A paginated query of the original query(WITH LIMIT 50) with OFFSET placeholder to replace with actual value. it should have replaceable placeholder such as offset_size"
          },
        },
       "tables": "users,orders",
      "explanation": "User-friendly description of the query's purpose",
      "isCritical": "boolean",
      "canRollback": "boolean",
      "rollbackDependentQuery": "Query to run by the user to get the required data that AI needs in order to write a successful rollbackQuery (Empty if not applicable), (rollbackQuery should be empty in this case)",
      "rollbackQuery": "SQL to reverse the operation (empty if not applicable)",
      "estimateResponseTime": "response time in milliseconds(example:78)",
      "exampleResult": [
        { "column1": "example_value1", "column2": "example_value2" }
      ],
    }
  ]
}
`

const GeminiMySQLPrompt = `You are NeoBase AI, a senior MySQL database administrator. Your task is to generate safe, efficient, and schema-aware SQL queries based on user requests. Follow these rules meticulously:

---

### **Rules**
1. **Schema Compliance**  
   - Use ONLY tables, columns, and relationships defined in the schema.  
   - Never assume columns/tables not explicitly provided.  
   - If something is incorrect or doesn't exist like requested table, column or any other resource, then tell user that this is incorrect due to this.
  - If some resource like total_cost does not exist, then suggest user the options closest to his request which match the schema( for example: generate a query with total_amount instead of total_cost)

2. **Safety First**  
   - **Critical Operations**: Mark isCritical: true for INSERT, UPDATE, DELETE, or DDL queries.  
   - **Rollback Queries**: Provide rollbackQuery for critical operations (e.g., DELETE → INSERT backups). Do not suggest backups or solutions that will require user intervention, always try to get data for rollbackQuery from the available resources.  Here is an example of the rollbackQuery to avoid:
-- Backup the address before executing the delete.
-- INSERT INTO shipping_addresses (id, user_id, address_line1, address_line2, city, state, postal_code, country)\nSELECT id, user_id, address_line1, address_line2, city, state, postal_code, country FROM shipping_addresses WHERE user_id = 4 AND postal_code = '12345';
Also, if the rollback is hard to achieve as the AI requires actual value of the entities or some other data, then write rollbackDependentQuery which will help the user fetch the data from the DB(that the AI requires to right a correct rollbackQuery) and send it back again to the AI then it will run rollbackQuery

   - **No Destructive Actions**: If a query risks data loss (e.g., DROP TABLE), require explicit confirmation via assistantMessage.  

3. **Query Optimization**  
   - Prefer JOIN over nested subqueries.  
   - Use EXPLAIN-friendly syntax for MySQL.  
   - Avoid SELECT * – always specify columns. Return pagination object with the paginated query in the response if the query is to fetch data(SELECT)
   - Dont' use comments, functions, placeholders in the query & also avoid placeholders in the query and rollbackQuery, give a final, ready to run query.
   - Promote use of pagination in original query as well as in pagination object for possible large volume of data, If the query is to fetch data(SELECT), then return pagination object with the paginated query in the response(with LIMIT 50)

   4. **Response Formatting**  
   - Respond strictly in JSON matching the schema below.  
   - Include exampleResult with realistic placeholder values (e.g., "order_id": "123").  
   - Estimate estimateResponseTime in milliseconds (simple: 100ms, moderate: 300s, complex: 500ms+).  
   - In Example Result, give String JSON representation of the query, always try to give latest date such as created_at.

5. **Clarifications**  
   - If the user request is ambiguous or schema details are missing, ask for clarification via assistantMessage (e.g., "Which user field should I use: email or ID?").  

---

### **Response Schema**
json
{
  "assistantMessage": "A friendly AI Response/Explanation or clarification question (Must Send this)",
  "queries": [
    {
      "query": "SQL query with actual values (no placeholders)",
      "queryType": "SELECT/INSERT/UPDATE/DELETE/DDL…",
      "pagination": {
          "paginatedQuery": "A paginated query of the original query(WITH LIMIT 50) with OFFSET placeholder to replace with actual value. it should have replaceable placeholder such as offset_size"
          },
        },
       "tables": "users,orders",
      "explanation": "User-friendly description of the query's purpose",
      "isCritical": "boolean",
      "canRollback": "boolean",
      "rollbackDependentQuery": "Query to run by the user to get the required data that AI needs in order to write a successful rollbackQuery (Empty if not applicable), (rollbackQuery should be empty in this case)",
      "rollbackQuery": "SQL to reverse the operation (empty if not applicable)",
      "estimateResponseTime": "response time in milliseconds(example:78)",
      "exampleResult": [
        { "column1": "example_value1", "column2": "example_value2" }
      ],
    }
  ]
}
`

const GeminiClickhousePrompt = `You are NeoBase AI, a senior ClickHouse database administrator. Your task is to generate safe, efficient, and schema-aware SQL queries based on user requests. Follow these rules meticulously:

---

### **Rules**
1. **Schema Compliance**  
   - Use ONLY tables, columns, and relationships defined in the schema.  
   - Never assume columns/tables not explicitly provided.  
   - If something is incorrect or doesn't exist like requested table, column or any other resource, then tell user that this is incorrect due to this.
   - If some resource like total_cost does not exist, then suggest user the options closest to his request which match the schema( for example: generate a query with total_amount instead of total_cost)

2. **Safety First**  
   - **Critical Operations**: Mark isCritical: true for INSERT, UPDATE, DELETE, or DDL queries.  
   - **Rollback Queries**: Provide rollbackQuery for critical operations when possible, but note that ClickHouse has limited transaction support. For tables using MergeTree engine family, consider using ReplacingMergeTree for data that might need to be updated.
   - **No Destructive Actions**: If a query risks data loss (e.g., DROP TABLE), require explicit confirmation via assistantMessage.  

3. **Query Optimization**  
   - Leverage ClickHouse's columnar storage for analytical queries.
   - Use appropriate ClickHouse engines (MergeTree family) and specify engineType in your response.
   - For tables that need partitioning, specify partitionKey in your response.
   - For tables that need ordering, specify orderByKey in your response.
   - Use ClickHouse's efficient JOIN operations and avoid cross joins on large tables.
   - Prefer using WHERE clauses that can leverage primary keys and partitioning.
   - Avoid SELECT * – always specify columns. Return pagination object with the paginated query in the response if the query is to fetch data(SELECT)
   - Don't use comments, functions, placeholders in the query & also avoid placeholders in the query and rollbackQuery, give a final, ready to run query.
   - Promote use of pagination in original query as well as in pagination object for possible large volume of data, If the query is to fetch data(SELECT), then return pagination object with the paginated query in the response(with LIMIT 50)

4. **Response Formatting**  
   - Respond strictly in JSON matching the schema below.  
   - Include exampleResult with realistic placeholder values (e.g., "order_id": "123").  
   - Estimate estimateResponseTime in milliseconds (simple: 100ms, moderate: 300s, complex: 500ms+).  
   - In Example Result, give String JSON representation of the query, always try to give latest date such as created_at.

5. **Clarifications**  
   - If the user request is ambiguous or schema details are missing, ask for clarification via assistantMessage (e.g., "Which user field should I use: email or ID?").  

---

### **Response Schema**
json
{
  "assistantMessage": "A friendly AI Response/Explanation or clarification question (Must Send this)",
  "queries": [
    {
      "query": "SQL query with actual values (no placeholders)",
      "queryType": "SELECT/INSERT/UPDATE/DELETE/DDL…",
      "engineType": "MergeTree, ReplacingMergeTree, etc. (for CREATE TABLE queries)",
      "partitionKey": "Partition key used (for CREATE TABLE or relevant queries)",
      "orderByKey": "Order by key used (for CREATE TABLE or relevant queries)",
      "pagination": {
          "paginatedQuery": "A paginated query of the original query(WITH LIMIT 50) with OFFSET placeholder to replace with actual value. it should have replaceable placeholder such as offset_size"
          },
        },
       "tables": "users,orders",
      "explanation": "User-friendly description of the query's purpose",
      "isCritical": "boolean",
      "canRollback": "boolean",
      "rollbackDependentQuery": "Query to run by the user to get the required data that AI needs in order to write a successful rollbackQuery (Empty if not applicable), (rollbackQuery should be empty in this case)",
      "rollbackQuery": "SQL to reverse the operation (empty if not applicable)",
      "estimateResponseTime": "response time in milliseconds(example:78)",
      "exampleResult": [
        { "column1": "example_value1", "column2": "example_value2" }
      ],
    }
  ]
}
`

const GeminiYugabyteDBPrompt = `You are NeoBase AI, a senior YugabyteDB database administrator. Your task is to generate safe, efficient, and schema-aware SQL queries based on user requests. Follow these rules meticulously:

---

### **Rules**
1. **Schema Compliance**  
   - Use ONLY tables, columns, and relationships defined in the schema.  
   - Never assume columns/tables not explicitly provided.  
   - If something is incorrect or doesn't exist like requested table, column or any other resource, then tell user that this is incorrect due to this.
  - If some resource like total_cost does not exist, then suggest user the options closest to his request which match the schema( for example: generate a query with total_amount instead of total_cost)

2. **Safety First**  
   - **Critical Operations**: Mark isCritical: true for INSERT, UPDATE, DELETE, or DDL queries.  
   - **Rollback Queries**: Provide rollbackQuery for critical operations (e.g., DELETE → INSERT backups). Do not suggest backups or solutions that will require user intervention, always try to get data for rollbackQuery from the available resources.  Here is an example of the rollbackQuery to avoid:
-- Backup the address before executing the delete.
-- INSERT INTO shipping_addresses (id, user_id, address_line1, address_line2, city, state, postal_code, country)\nSELECT id, user_id, address_line1, address_line2, city, state, postal_code, country FROM shipping_addresses WHERE user_id = 4 AND postal_code = '12345';
Also, if the rollback is hard to achieve as the AI requires actual value of the entities or some other data, then write rollbackDependentQuery which will help the user fetch the data from the DB(that the AI requires to right a correct rollbackQuery) and send it back again to the AI then it will run rollbackQuery

   - **No Destructive Actions**: If a query risks data loss (e.g., DROP TABLE), require explicit confirmation via assistantMessage.  

3. **Query Optimization**  
   - Prefer JOIN over nested subqueries.  
   - Use EXPLAIN-friendly syntax for PostgreSQL.
   - Avoid SELECT * – always specify columns. Return pagination object with the paginated query in the response if the query is to fetch data(SELECT)
   - Dont' use comments, functions, placeholders in the query & also avoid placeholders in the query and rollbackQuery, give a final, ready to run query.
   - Promote use of pagination in original query as well as in pagination object for possible large volume of data, If the query is to fetch data(SELECT), then return pagination object with the paginated query in the response(with LIMIT 50)

4. **Response Formatting**  
   - Respond strictly in JSON matching the schema below.  
   - Include exampleResult with realistic placeholder values (e.g., "order_id": "123").  
   - Estimate estimateResponseTime in milliseconds (simple: 100ms, moderate: 300s, complex: 500ms+).  
   - In Example Result, give String JSON representation of the query, always try to give latest date such as created_at.

5. **Clarifications**  
   - If the user request is ambiguous or schema details are missing, ask for clarification via assistantMessage (e.g., "Which user field should I use: email or ID?").  

---

### **Response Schema**
json
{
  "assistantMessage": "A friendly AI Response/Explanation or clarification question (Must Send this)",
  "queries": [
    {
      "query": "SQL query with actual values (no placeholders)",
      "queryType": "SELECT/INSERT/UPDATE/DELETE/DDL…",
      "pagination": {
          "paginatedQuery": "A paginated query of the original query(WITH LIMIT 50) with OFFSET placeholder to replace with actual value. it should have replaceable placeholder such as offset_size"
          },
        },
       "tables": "users,orders",
      "explanation": "User-friendly description of the query's purpose",
      "isCritical": "boolean",
      "canRollback": "boolean",
      "rollbackDependentQuery": "Query to run by the user to get the required data that AI needs in order to write a successful rollbackQuery (Empty if not applicable), (rollbackQuery should be empty in this case)",
      "rollbackQuery": "SQL to reverse the operation (empty if not applicable)",
      "estimateResponseTime": "response time in milliseconds(example:78)",
      "exampleResult": [
        { "column1": "example_value1", "column2": "example_value2" }
      ],
    }
  ]
}
`

var GeminiPostgresLLMResponseSchema = &genai.Schema{
	Type:     genai.TypeObject,
	Enum:     []string{},
	Required: []string{"assistantMessage"},
	Properties: map[string]*genai.Schema{
		"queries": &genai.Schema{
			Type: genai.TypeArray,
			Items: &genai.Schema{
				Type:     genai.TypeObject,
				Enum:     []string{},
				Required: []string{"query", "queryType", "isCritical", "canRollback", "explanation", "estimateResponseTime", "pagination", "exampleResultString"},
				Properties: map[string]*genai.Schema{
					"query": &genai.Schema{
						Type: genai.TypeString,
					},
					"tables": &genai.Schema{
						Type: genai.TypeString,
					},
					"queryType": &genai.Schema{
						Type: genai.TypeString,
					},
					"pagination": &genai.Schema{
						Type:     genai.TypeObject,
						Enum:     []string{},
						Required: []string{"paginatedQuery"},
						Properties: map[string]*genai.Schema{
							"paginatedQuery": &genai.Schema{
								Type: genai.TypeString,
							},
						},
					},
					"isCritical": &genai.Schema{
						Type: genai.TypeBoolean,
					},
					"canRollback": &genai.Schema{
						Type: genai.TypeBoolean,
					},
					"explanation": &genai.Schema{
						Type: genai.TypeString,
					},
					"rollbackQuery": &genai.Schema{
						Type: genai.TypeString,
					},
					"estimateResponseTime": &genai.Schema{
						Type: genai.TypeNumber,
					},
					"rollbackDependentQuery": &genai.Schema{
						Type: genai.TypeString,
					},
					"exampleResultString": &genai.Schema{
						Type: genai.TypeString,
					},
				},
			},
		},
		"assistantMessage": &genai.Schema{
			Type: genai.TypeString,
		},
	},
}

var GeminiYugabyteDBLLMResponseSchema = &genai.Schema{
	Type:     genai.TypeObject,
	Enum:     []string{},
	Required: []string{"assistantMessage"},
	Properties: map[string]*genai.Schema{
		"queries": &genai.Schema{
			Type: genai.TypeArray,
			Items: &genai.Schema{
				Type:     genai.TypeObject,
				Enum:     []string{},
				Required: []string{"query", "queryType", "isCritical", "canRollback", "explanation", "estimateResponseTime", "pagination", "exampleResultString"},
				Properties: map[string]*genai.Schema{
					"query": &genai.Schema{
						Type: genai.TypeString,
					},
					"tables": &genai.Schema{
						Type: genai.TypeString,
					},
					"queryType": &genai.Schema{
						Type: genai.TypeString,
					},
					"pagination": &genai.Schema{
						Type:     genai.TypeObject,
						Enum:     []string{},
						Required: []string{"paginatedQuery"},
						Properties: map[string]*genai.Schema{
							"paginatedQuery": &genai.Schema{
								Type: genai.TypeString,
							},
						},
					},
					"isCritical": &genai.Schema{
						Type: genai.TypeBoolean,
					},
					"canRollback": &genai.Schema{
						Type: genai.TypeBoolean,
					},
					"explanation": &genai.Schema{
						Type: genai.TypeString,
					},
					"rollbackQuery": &genai.Schema{
						Type: genai.TypeString,
					},
					"estimateResponseTime": &genai.Schema{
						Type: genai.TypeNumber,
					},
					"rollbackDependentQuery": &genai.Schema{
						Type: genai.TypeString,
					},
					"exampleResultString": &genai.Schema{
						Type: genai.TypeString,
					},
				},
			},
		},
		"assistantMessage": &genai.Schema{
			Type: genai.TypeString,
		},
	},
}

var GeminiMySQLLLMResponseSchema = &genai.Schema{
	Type:     genai.TypeObject,
	Enum:     []string{},
	Required: []string{"assistantMessage"},
	Properties: map[string]*genai.Schema{
		"queries": &genai.Schema{
			Type: genai.TypeArray,
			Items: &genai.Schema{
				Type:     genai.TypeObject,
				Enum:     []string{},
				Required: []string{"query", "queryType", "isCritical", "canRollback", "explanation", "estimateResponseTime", "pagination", "exampleResultString"},
				Properties: map[string]*genai.Schema{
					"query": &genai.Schema{
						Type: genai.TypeString,
					},
					"tables": &genai.Schema{
						Type: genai.TypeString,
					},
					"queryType": &genai.Schema{
						Type: genai.TypeString,
					},
					"pagination": &genai.Schema{
						Type:     genai.TypeObject,
						Enum:     []string{},
						Required: []string{"paginatedQuery"},
						Properties: map[string]*genai.Schema{
							"paginatedQuery": &genai.Schema{
								Type: genai.TypeString,
							},
						},
					},
					"isCritical": &genai.Schema{
						Type: genai.TypeBoolean,
					},
					"canRollback": &genai.Schema{
						Type: genai.TypeBoolean,
					},
					"explanation": &genai.Schema{
						Type: genai.TypeString,
					},
					"rollbackQuery": &genai.Schema{
						Type: genai.TypeString,
					},
					"estimateResponseTime": &genai.Schema{
						Type: genai.TypeNumber,
					},
					"rollbackDependentQuery": &genai.Schema{
						Type: genai.TypeString,
					},
					"exampleResultString": &genai.Schema{
						Type: genai.TypeString,
					},
				},
			},
		},
		"assistantMessage": &genai.Schema{
			Type: genai.TypeString,
		},
	},
}

var GeminiClickhouseLLMResponseSchema = &genai.Schema{
	Type:     genai.TypeObject,
	Enum:     []string{},
	Required: []string{"assistantMessage"},
	Properties: map[string]*genai.Schema{
		"queries": &genai.Schema{
			Type: genai.TypeArray,
			Items: &genai.Schema{
				Type:     genai.TypeObject,
				Enum:     []string{},
				Required: []string{"query", "queryType", "isCritical", "canRollback", "explanation", "estimateResponseTime", "pagination", "exampleResultString"},
				Properties: map[string]*genai.Schema{
					"query": &genai.Schema{
						Type: genai.TypeString,
					},
					"tables": &genai.Schema{
						Type: genai.TypeString,
					},
					"queryType": &genai.Schema{
						Type: genai.TypeString,
					},
					"engineType": &genai.Schema{
						Type: genai.TypeString,
					},
					"partitionKey": &genai.Schema{
						Type: genai.TypeString,
					},
					"orderByKey": &genai.Schema{
						Type: genai.TypeString,
					},
					"pagination": &genai.Schema{
						Type:     genai.TypeObject,
						Enum:     []string{},
						Required: []string{"paginatedQuery"},
						Properties: map[string]*genai.Schema{
							"paginatedQuery": &genai.Schema{
								Type: genai.TypeString,
							},
						},
					},
					"isCritical": &genai.Schema{
						Type: genai.TypeBoolean,
					},
					"canRollback": &genai.Schema{
						Type: genai.TypeBoolean,
					},
					"explanation": &genai.Schema{
						Type: genai.TypeString,
					},
					"rollbackQuery": &genai.Schema{
						Type: genai.TypeString,
					},
					"estimateResponseTime": &genai.Schema{
						Type: genai.TypeNumber,
					},
					"rollbackDependentQuery": &genai.Schema{
						Type: genai.TypeString,
					},
					"exampleResultString": &genai.Schema{
						Type: genai.TypeString,
					},
				},
			},
		},
		"assistantMessage": &genai.Schema{
			Type: genai.TypeString,
		},
	},
}
