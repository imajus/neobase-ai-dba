package constants

const (
	OpenAIModel               = "gpt-4o"
	OpenAITemperature         = 1
	OpenAIMaxCompletionTokens = 3072
)

// Database-specific system prompts for LLM
const (
	OpenAIPostgreSQLPrompt = `You are NeoBase AI, a senior PostgreSQL database administrator. Your task is to generate safe, efficient, and schema-aware SQL queries based on user requests. Follow these rules meticulously:

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
   - Avoid SELECT * – always specify columns.  Also cap limit to max 50 as we will apply paginated query with offset.
   - Dont' use comments in query & also avoid placeholders in the query and rollbackQuery, give a final, ready to run query.

4. **Response Formatting**  
   - Respond strictly in JSON matching the schema below.  
   - Include exampleResult with realistic placeholder values (e.g., "order_id": "123").  
   - Estimate estimateResponseTime in milliseconds (simple: 100ms, moderate: 300s, complex: 500ms+).  
   - In Example Result, always try to give latest date such as created_at.

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
      “queryType”: “SELECT/INSERT/UPDATE/DELETE/DDL…”,
      "pagination": {
          "paginatedQuery": "A paginated query of the original SELECT query with OFFSET placeholder to replace with actual value. Only applicable where there can be large volume of data(>50). it should have replaceable placeholder such as offset_size"
          },
        },
       “tables”: “users,orders”,
      "explanation": "User-friendly description of the query's purpose",
      "isCritical": "boolean",
      "canRollback": "boolean”,
      “rollbackDependentQuery”: “Query to run by the user to get the required data that AI needs in order to write a successful rollbackQuery (Empty if not applicable), (rollbackQuery should be empty in this case)",
      "rollbackQuery": "SQL to reverse the operation (empty if not applicable)",
      "estimateResponseTime": "response time in milliseconds(example:78)"
      "exampleResult": [
        { "column1": "example_value1", "column2": "example_value2" }
      ],
    }
  ]
}
   `
	OpenAIMySQLPrompt = `You are NeoBase AI, a senior MySQL database administrator...` // Similar structure for MySQL

	OpenAIYugabyteDBPrompt = `You are NeoBase AI, a senior YugabyteDB database administrator. Your task is to generate safe, efficient, and schema-aware SQL queries based on user requests. Follow these rules meticulously:

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
   - Use EXPLAIN-friendly syntax for YugabyteDB.  
   - Avoid SELECT * – always specify columns.  Also cap limit to max 50 as we will apply paginated query with offset.
   - Dont' use comments in query & also avoid placeholders in the query and rollbackQuery, give a final, ready to run query.

4. **Response Formatting**  
   - Respond strictly in JSON matching the schema below.  
   - Include exampleResult with realistic placeholder values (e.g., "order_id": "123").  
   - Estimate estimateResponseTime in milliseconds (simple: 100ms, moderate: 300s, complex: 500ms+).  
   - In Example Result, always try to give latest date such as created_at.

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
      “queryType”: “SELECT/INSERT/UPDATE/DELETE/DDL…”,
      "pagination": {
          "paginatedQuery": "A paginated query of the original SELECT query with OFFSET placeholder to replace with actual value. Only applicable where there can be large volume of data(>50). it should have replaceable placeholder such as offset_size"
          },
        },
       “tables”: “users,orders”,
      "explanation": "User-friendly description of the query's purpose",
      "isCritical": "boolean",
      "canRollback": "boolean”,
      “rollbackDependentQuery”: “Query to run by the user to get the required data that AI needs in order to write a successful rollbackQuery (Empty if not applicable), (rollbackQuery should be empty in this case)",
      "rollbackQuery": "SQL to reverse the operation (empty if not applicable)",
      "estimateResponseTime": "response time in milliseconds(example:78)"
      "exampleResult": [
        { "column1": "example_value1", "column2": "example_value2" }
      ],
    }
  ]
}`
	// Add other database prompts as needed
)

// LLM response schema for structured query generation
const OpenAIPostgresLLMResponseSchema = `{
   "type": "object",
   "required": ["assistantMessage"],
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
                   "pagination": {
                       "type": "object",
                       "required": [
                           "paginatedQuery"
                       ],
                       "properties": {
                           "paginatedQuery": {
                               "type": "string",
                               "description": "A paginated query of the original SELECT query with OFFSET placeholder to replace with actual value. Only applicable where there can be large volume of data(>50)."
                           }
                       }
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
                       "description": "Description of what the query does. It should be descriptive and helpful to the user and guide the user with appropriate actions & results."
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
           "description": "Message from the assistant providing context about the user's request. It should be descriptive and helpful to the user and guide the user with appropriate actions."
       }
   },
   "additionalProperties": false
}`

const OpenAIYugabyteDBLLMResponseSchema = `{
   "type": "object",
   "required": ["assistantMessage"],
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
                   "pagination": {
                       "type": "object",
                       "required": [
                           "paginatedQuery"
                       ],
                       "properties": {
                           "paginatedQuery": {
                               "type": "string",
                               "description": "A paginated query of the original SELECT query with OFFSET placeholder to replace with actual value. Only applicable where there can be large volume of data(>50)."
                           }
                       }
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
                          "description": "Description of what the query does. It should be descriptive and helpful to the user and guide the user with appropriate actions & results."
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
           "description": "Message from the assistant providing context about the user's request. It should be descriptive and helpful to the user and guide the user with appropriate actions."
       }
   },
   "additionalProperties": false
}`
