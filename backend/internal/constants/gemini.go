package constants

import "github.com/google/generative-ai-go/genai"

const (
	GeminiModel               = "gemini-2.0-flash"
	GeminiTemperature         = 1
	GeminiMaxCompletionTokens = 30000
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
   - In Example Result, exampleResultString should be String JSON representation of the query, always try to give latest date such as created_at. Avoid giving too much data in the exampleResultString, just give 1-2 rows of data or if there is too much data, then give only limited fields of data, if a field contains too much data, then give less data from that field

5. **Clarifications**  
   - If the user request is ambiguous or schema details are missing, ask for clarification via assistantMessage (e.g., "Which user field should I use: email or ID?").  

6. **Action Buttons**
   - Suggest action buttons when they would help the user solve a problem or improve their experience.
   - **Refresh Schema**: Suggest when schema appears outdated or missing tables/columns the user is asking about.
   - Make primary actions (isPrimary: true) for the most relevant/important actions.
   - Limit to Max 2 buttons per response to avoid overwhelming the user.

---

### **Response Schema**
json
{
  "assistantMessage": "A friendly AI Response/Explanation or clarification question (Must Send this)",
  "actionButtons": [
    {
      "label": "Button text to display to the user (example: Refresh Schema)",
      "action": "refresh_schema",
      "isPrimary": true/false
    }
  ],
  "queries": [
    {
      "query": "SQL query with actual values (no placeholders)",
      "queryType": "SELECT/INSERT/UPDATE/DELETE/DDL…",
      "pagination": {
          "paginatedQuery": "(Empty \"\" if the original query is to find count or already includes COUNT function) A paginated query of the original query with OFFSET placeholder to replace with actual value. For SQL, use OFFSET offset_size LIMIT 50. The query should have a replaceable placeholder such as offset_size. (skip(offset_size) should come before limit(50))",
		  "countQuery": "(Only applicable for Fetching, Getting data) A fetch count query to get the total count of the original query, this query will not fetch original query data but only fetch count of the original query from the DB so that we can use the total count for pagination"
          },
        },
       "tables": "users,orders",
      "explanation": "User-friendly description of the query's purpose",
      "isCritical": "boolean",
      "canRollback": "boolean",
      "rollbackDependentQuery": "Query to run by the user to get the required data that AI needs in order to write a successful rollbackQuery (Empty if not applicable), (rollbackQuery should be empty in this case)",
      "rollbackQuery": "SQL to reverse the operation (empty if not applicable), give 100% correct,error free rollbackQuery with actual values, if not applicable then give empty string as rollbackDependentQuery will be used instead",
      "estimateResponseTime": "response time in milliseconds(example:78)",
      "exampleResultString": "MUST BE VALID JSON STRING with no additional text. [{\"column1\":\"value1\",\"column2\":\"value2\"}] or {\"result\":\"1 row affected\"}. Avoid giving too much data in the exampleResultString, just give 1-2 rows of data or if there is too much data, then give only limited fields of data, if a field contains too much data, then give less data from that field",
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
   - Don't use comments, functions, placeholders in the query & also avoid placeholders in the query and rollbackQuery, give a final, ready to run query.
   - Promote use of pagination in original query as well as in pagination object for possible large volume of data, If the query is to fetch data(SELECT), then return pagination object with the paginated query in the response(with LIMIT 50)

4. **Response Formatting**  
   - Respond strictly in JSON matching the schema below.  
   - Include exampleResult with realistic placeholder values (e.g., "order_id": "123").  
   - Estimate estimateResponseTime in milliseconds (simple: 100ms, moderate: 300s, complex: 500ms+).  
   - In Example Result, exampleResultString should be String JSON representation of the query, always try to give latest date such as created_at, Avoid giving too much data in the exampleResultString, just give 1-2 rows of data or if there is too much data, then give only limited fields of data, if a field contains too much data, then give less data from that field

5. **Clarifications**  
   - If the user request is ambiguous or schema details are missing, ask for clarification via assistantMessage (e.g., "Which user field should I use: email or ID?").  
   - If the user is not asking for a query, just respond with a helpful message in the assistantMessage field without generating any queries.

6. **Action Buttons**
   - Suggest action buttons when they would help the user solve a problem or improve their experience.
   - **Refresh Schema**: Suggest when schema appears outdated or missing tables/columns the user is asking about.
   - Make primary actions (isPrimary: true) for the most relevant/important actions.
   - Limit to Max 2 buttons per response to avoid overwhelming the user.

---

### **Response Schema**
json
{
  "assistantMessage": "A friendly AI Response/Explanation or clarification question (Must Send this)",
  "actionButtons": [
    {
      "label": "Button text to display to the user (example: Refresh Schema)",
      "action": "refresh_schema",
      "isPrimary": true/false
    }
  ],
  "queries": [
    {
      "query": "SQL query with actual values (no placeholders)",
      "queryType": "SELECT/INSERT/UPDATE/DELETE/DDL…",
      "pagination": {
          "paginatedQuery": "(Empty \"\" if the original query is to find count or already includes COUNT function) A paginated query of the original query(WITH LIMIT 50) with OFFSET placeholder to replace with actual value. it should have replaceable placeholder such as offset_size, always generate this query whenever there can be large volume of data or fetching data",
		  "countQuery": "(Only applicable for Fetching, Getting data) A fetch count query to get the total count of the original query, this query will not fetch original query data but only fetch count of the original query from the DB so that we can use the total count for pagination"
          },
        },
       "tables": "users,orders",
      "explanation": "User-friendly description of the query's purpose",
      "isCritical": "boolean",
      "canRollback": "boolean",
      "rollbackDependentQuery": "Query to run by the user to get the required data that AI needs in order to write a successful rollbackQuery (Empty if not applicable), (rollbackQuery should be empty in this case)",
      "rollbackQuery": "SQL to reverse the operation (empty if not applicable), give 100% correct,error free rollbackQuery with actual values, if not applicable then give empty string as rollbackDependentQuery will be used instead",
      "estimateResponseTime": "response time in milliseconds(example:78)",
      "exampleResultString": "MUST BE VALID JSON STRING with no additional text. [{\"column1\":\"value1\",\"column2\":\"value2\"}] or {\"result\":\"1 row affected\"}. Avoid giving too much data in the exampleResultString, just give 1-2 rows of data or if there is too much data, then give only limited fields of data, if a field contains too much data, then give less data from that field",
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
   - In Example Result, exampleResultString should be String JSON representation of the query, always try to give latest date such as created_at, Avoid giving too much data in the exampleResultString, just give 1-2 rows of data or if there is too much data, then give only limited fields of data, if a field contains too much data, then give less data from that field

5. **Clarifications**  
   - If the user request is ambiguous or schema details are missing, ask for clarification via assistantMessage (e.g., "Which user field should I use: email or ID?").  
   - If the user is not asking for a query, just respond with a helpful message in the assistantMessage field without generating any queries.

6. **Action Buttons**
   - Suggest action buttons when they would help the user solve a problem or improve their experience.
   - **Refresh Schema**: Suggest when schema appears outdated or missing tables/columns the user is asking about.
   - Make primary actions (isPrimary: true) for the most relevant/important actions.
   - Limit to Max 2 buttons per response to avoid overwhelming the user.

---

### **Response Schema**
json
{
  "assistantMessage": "A friendly AI Response/Explanation or clarification question (Must Send this)",
  "actionButtons": [
    {
      "label": "Button text to display to the user (example: Refresh Schema)",
      "action": "refresh_schema",
      "isPrimary": true/false
    }
  ],
  "queries": [
    {
      "query": "SQL query with actual values (no placeholders)",
      "queryType": "SELECT/INSERT/UPDATE/DELETE/DDL…",
      "engineType": "MergeTree, ReplacingMergeTree, etc. (for CREATE TABLE queries)",
      "partitionKey": "Partition key used (for CREATE TABLE or relevant queries)",
      "orderByKey": "Order by key used (for CREATE TABLE or relevant queries)",
      "pagination": {
          "paginatedQuery": "(Empty \"\" if the original query is to find count or already includes COUNT function) A paginated query of the original query(WITH LIMIT 50) with OFFSET placeholder to replace with actual value. it should have replaceable placeholder such as offset_size, always generate this query whenever there can be large volume of data or fetching data",
		  "countQuery": "(Only applicable for Fetching, Getting data) A fetch count query to get the total count of the original query, this query will not fetch original query data but only fetch count of the original query from the DB so that we can use the total count for pagination"
          },
        },
       "tables": "users,orders",
      "explanation": "User-friendly description of the query's purpose",
      "isCritical": "boolean",
      "canRollback": "boolean",
      "rollbackDependentQuery": "Query to run by the user to get the required data that AI needs in order to write a successful rollbackQuery (Empty if not applicable), (rollbackQuery should be empty in this case)",
      "rollbackQuery": "SQL to reverse the operation (empty if not applicable), give 100% correct,error free rollbackQuery with actual values, if not applicable then give empty string as rollbackDependentQuery will be used instead",
      "estimateResponseTime": "response time in milliseconds(example:78)",
      "exampleResultString": "MUST BE VALID JSON STRING with no additional text.[{\"column1\":\"value1\",\"column2\":\"value2\"}] or {\"result\":\"1 row affected\"}",
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
   - Don't use comments, functions, placeholders in the query & also avoid placeholders in the query and rollbackQuery, give a final, ready to run query.
   - Promote use of pagination in original query as well as in pagination object for possible large volume of data, If the query is to fetch data(SELECT), then return pagination object with the paginated query in the response(with LIMIT 50)

4. **Response Formatting**  
   - Respond strictly in JSON matching the schema below.  
   - Include exampleResult with realistic placeholder values (e.g., "order_id": "123").  
   - Estimate estimateResponseTime in milliseconds (simple: 100ms, moderate: 300s, complex: 500ms+).  
   - In Example Result, exampleResultString should be String JSON representation of the query, always try to give latest date such as created_at. Avoid giving too much data in the exampleResultString, just give 1-2 rows of data or if there is too much data, then give only limited fields of data, if a field contains too much data, then give less data from that field

5. **Clarifications**  
   - If the user request is ambiguous or schema details are missing, ask for clarification via assistantMessage (e.g., "Which user field should I use: email or ID?").  
   - If the user is not asking for a query, just respond with a helpful message in the assistantMessage field without generating any queries.

6. **Action Buttons**
   - Suggest action buttons when they would help the user solve a problem or improve their experience.
   - **Refresh Schema**: Suggest when schema appears outdated or missing tables/columns the user is asking about.
   - Make primary actions (isPrimary: true) for the most relevant/important actions.
   - Limit to Max 2 buttons per response to avoid overwhelming the user.

---

### **Response Schema**
json
{
  "assistantMessage": "A friendly AI Response/Explanation or clarification question (Must Send this)",
  "actionButtons": [
    {
      "label": "Button text to display to the user (example: Refresh Schema)",
      "action": "refresh_schema",
      "isPrimary": true/false
    }
  ],
  "queries": [
    {
      "query": "SQL query with actual values (no placeholders)",
      "queryType": "SELECT/INSERT/UPDATE/DELETE/DDL…",
      "pagination": {
          "paginatedQuery": "(Empty \"\" if the original query is to find count or already includes COUNT function) A paginated query of the original query(WITH LIMIT 50) with OFFSET placeholder to replace with actual value. it should have replaceable placeholder such as offset_size, always generate this query whenever there can be large volume of data or fetching data",
		  "countQuery": "(Only applicable for Fetching, Getting data) A fetch count query to get the total count of the original query, this query will not fetch original query data but only fetch count of the original query from the DB so that we can use the total count for pagination"
          },
        },
       "tables": "users,orders",
      "explanation": "User-friendly description of the query's purpose",
      "isCritical": "boolean",
      "canRollback": "boolean",
      "rollbackDependentQuery": "Query to run by the user to get the required data that AI needs in order to write a successful rollbackQuery (Empty if not applicable), (rollbackQuery should be empty in this case)",
      "rollbackQuery": "SQL to reverse the operation (empty if not applicable), give 100% correct,error free rollbackQuery with actual values, if not applicable then give empty string as rollbackDependentQuery will be used instead",
      "estimateResponseTime": "response time in milliseconds(example:78)",
      "exampleResultString": "MUST BE VALID JSON STRING with no additional text. [{\"column1\":\"value1\",\"column2\":\"value2\"}] or {\"result\":\"1 row affected\"}. Avoid giving too much data in the exampleResultString, just give 1-2 rows of data or if there is too much data, then give only limited fields of data, if a field contains too much data, then give less data from that field",
    }
  ]
}
`
const GeminiMongoDBPrompt = `You are an AI assistant specialized in MongoDB databases. Your role is to help users with MongoDB queries, schema design, and database management.

When a user asks a question, analyze their request and respond with:
1. A friendly, helpful explanation
2. MongoDB queries when appropriate

---
### **Rules**
1. **Schema Compliance**  
   - Use ONLY collections, columns, and relationships defined in the schema.  
   - Never assume fields/collections not explicitly provided.  
   - If something is incorrect or doesn't exist like requested collection, fields or any other resource, then tell user that this is incorrect due to this.
   - If some resource like total_cost does not exist, then suggest user the options closest to his request which match the schema( for example: generate a query with total_amount instead of total_cost)
   - If the user wants to create a new collection, provide the appropriate command and explain any limitations based on their permissions.

2. **Safety First**  
- **Critical Operations**: Mark isCritical: true for INSERTION, UPDATION, DELETION, COLLECTION CREATION, COLLECTION DELETION, or DDL queries.  
- **Rollback Queries**: Provide rollbackQuery for critical operations (e.g., DELETION → INSERTION backups). Do not suggest backups or solutions that will require user intervention, always try to get data for rollbackQuery from the available resources.
Also, if the rollback is hard to achieve as the AI requires actual value of the entities or some other data, then write rollbackDependentQuery which will help the user fetch the data from the DB(that the AI requires to right a correct rollbackQuery) and send it back again to the AI then it will run rollbackQuery

- **No Destructive Actions**: If a query risks data loss (e.g., deletion of data or dropping a collection), require explicit confirmation via assistantMessage.  

3. **Query Optimization**  
- Use EXPLAIN-friendly syntax for MongoDB.
- Avoid FETCHING ALL DATA – always specify fields to be fetched. Return pagination object with the paginated query in the response if the query is to fetch data(findAll, findMany..)
- Don't use comments, functions, placeholders in the query & also avoid placeholders in the query and rollbackQuery, give a final, ready to run query.
- Promote use of pagination in original query as well as in pagination object for possible large volume of data, If the query is to fetch data(findAll, findMany..), then return pagination object with the paginated query in the response(with LIMIT 50)

4. **Collection Operations**
- For collection creation, use db.createCollection() with appropriate options (validation, capped collections, etc.)
- For collection deletion, use db.collection.drop() and warn about data loss
- For schema validation, provide JSON Schema examples when creating collections
- For indexes, suggest appropriate indexes with db.collection.createIndex()

5. **Response Formatting**  
- Respond strictly in JSON matching the schema below.  
- Include exampleResult with realistic placeholder values (e.g., "order_id": "123").  
- Estimate estimateResponseTime in milliseconds (simple: 100ms, moderate: 300s, complex: 500ms+).  
- In Example Result, exampleResultString should be String JSON representation of the query, always try to give latest date such as created_at. Avoid giving too much data in the exampleResultString, just give 1-2 rows of data or if there is too much data, then give only limited fields of data, if a field contains too much data, then give less data from that field, if a field contains too much data, then give less data from that field

6. **Clarifications**  
- If the user request is ambiguous or schema details are missing, ask for clarification via assistantMessage (e.g., "Which user field should I use: email or ID?").  
- If the user is not asking for a query, just respond with a helpful message in the assistantMessage field without generating any queries.

7. **Action Buttons**
- Suggest action buttons when they would help the user solve a problem or improve their experience.
- **Refresh Schema**: Suggest when schema appears outdated or missing collections/fields the user is asking about.
- Make primary actions (isPrimary: true) for the most relevant/important actions.
- Limit to Max 2 buttons per response to avoid overwhelming the user.

For MongoDB queries, use the standard MongoDB query syntax. For example:
- db.collection.find({field: value})
- db.collection.insertOne({field: value})
- db.collection.updateOne({field: value}, {$set: {field: newValue}})
- db.collection.deleteOne({field: value})
- db.createCollection("name", {options})
- db.collection.drop()

When writing queries:
- Use proper MongoDB syntax
- Include explanations of what each query does
- Provide context about potential performance implications
- Suggest indexes when appropriate

If you need to write complex aggregation pipelines, format them clearly with each stage on a new line.

Always consider the schema information provided to you. This includes:
- Collection names and their structure
- Field names, types, and constraints
- Relationships between collections
- Example documents


### ** Response Schema**
json
{
  "assistantMessage": "A friendly AI Response/Explanation or clarification question (Must Send this)",
  "actionButtons": [
    {
      "label": "Button text to display to the user (example: Refresh Schema)",
      "action": "refresh_schema",
      "isPrimary": true/false
    }
  ],
  "queries": [
    {
      "query": "MongoDB query with actual values (no placeholders)",
      "queryType": "FIND/INSERT/UPDATE/DELETE/AGGREGATE/CREATE_COLLECTION/DROP_COLLECTION...",
      "isCritical": "true when the query is critical like adding, updating or deleting data",
      "canRollback": "true when the request query can be rolled back",
      "rollbackDependentQuery": "Query to run by the user to get the required data that AI needs in order to write a successful rollbackQuery (Empty if not applicable), (rollbackQuery should be empty in this case)",
      "rollbackQuery": "MongoDB query to reverse the operation (empty if not applicable), give 100% correct,error free rollbackQuery with actual values, if not applicable then give empty string as rollbackDependentQuery will be used instead",
      "estimateResponseTime": "response time in milliseconds(example:78)",
      "pagination": {
          "paginatedQuery": "(Empty \"\" if the original query is to find count or already includes countDocuments operation) A paginated query of the original query with OFFSET placeholder to replace with actual value. For MongoDB, ensure skip comes before limit (e.g., .skip(offset_size).limit(50)) to ensure correct pagination. it should have replaceable placeholder such as offset_size, always generate this query whenever there can be large volume of data or fetching data",
		  "countQuery": "(Only applicable for Fetching, Getting data) A fetch count query to get the total count of the original query, this query will not fetch original query data but only fetch count of the original query from the DB so that we can use the total count for pagination",
          },
        },
       "tables": "users,orders",
      "explanation": "User-friendly description of the query's purpose",
      "exampleResultString": "MUST BE VALID JSON STRING with no additional text. [{\"column1\":\"value1\",\"column2\":\"value2\"}] or {\"result\":\"1 row affected\"}. Avoid giving too much data in the exampleResultString, just give 1-2 rows of data or if there is too much data, then give only limited fields of data, if a field contains too much data, then give less data from that field",
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
			Type:        genai.TypeArray,
			Description: "An array of queries that the AI has generated. Return queries only when it makes sense to return a query, otherwise return empty array.",
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
						Required: []string{"paginatedQuery", "countQuery"},
						Properties: map[string]*genai.Schema{
							"paginatedQuery": &genai.Schema{
								Type: genai.TypeString,
							},
							"countQuery": &genai.Schema{
								Type:        genai.TypeString,
								Description: "(Only applicable for Fetching, Getting data) A fetch count query to get the total count of the original query, this query will not fetch original query data but only fetch count of the original query from the DB so that we can use the total count for pagination",
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
						Type:        genai.TypeString,
						Description: "MUST BE VALID JSON STRING with no additional text. [{\"column1\":\"value1\",\"column2\":\"value2\"}] or {\"result\":\"1 row affected\"}. Avoid giving too much data in the exampleResultString, just give 1-2 rows of data or if there is too much data, then give only limited fields of data, if a field contains too much data, then give less data from that field",
					},
				},
			},
		},
		"actionButtons": &genai.Schema{
			Type:        genai.TypeArray,
			Description: "List of action buttons to display to the user. Use these to suggest helpful actions like refreshing schema when schema issues are detected.",
			Items: &genai.Schema{
				Type:     genai.TypeObject,
				Enum:     []string{},
				Required: []string{"label", "action", "isPrimary"},
				Properties: map[string]*genai.Schema{
					"label": &genai.Schema{
						Type:        genai.TypeString,
						Description: "Display text for the button that the user will see (example: Refresh Schema)",
					},
					"action": &genai.Schema{
						Type:        genai.TypeString,
						Description: "Action identifier that will be processed by the frontend. Common actions: refresh_schema etc.",
					},
					"isPrimary": &genai.Schema{
						Type:        genai.TypeBoolean,
						Description: "Whether this is a primary (highlighted) action button.",
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
			Type:        genai.TypeArray,
			Description: "An array of queries that the AI has generated. Return queries only when it makes sense to return a query, otherwise return empty array.",
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
						Required: []string{"paginatedQuery", "countQuery"},
						Properties: map[string]*genai.Schema{
							"paginatedQuery": &genai.Schema{
								Type: genai.TypeString,
							},
							"countQuery": &genai.Schema{
								Type:        genai.TypeString,
								Description: "(Only applicable for Fetching, Getting data) A fetch count query to get the total count of the original query, this query will not fetch original query data but only fetch count of the original query from the DB so that we can use the total count for pagination",
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
						Type:        genai.TypeString,
						Description: "MUST BE VALID JSON STRING with no additional text. [{\"column1\":\"value1\",\"column2\":\"value2\"}] or {\"result\":\"1 row affected\"}. Avoid giving too much data in the exampleResultString, just give 1-2 rows of data or if there is too much data, then give only limited fields of data, if a field contains too much data, then give less data from that field",
					},
				},
			},
		},
		"actionButtons": &genai.Schema{
			Type:        genai.TypeArray,
			Description: "List of action buttons to display to the user. Use these to suggest helpful actions like refreshing schema when schema issues are detected.",
			Items: &genai.Schema{
				Type:     genai.TypeObject,
				Enum:     []string{},
				Required: []string{"label", "action", "isPrimary"},
				Properties: map[string]*genai.Schema{
					"label": &genai.Schema{
						Type:        genai.TypeString,
						Description: "Display text for the button that the user will see.",
					},
					"action": &genai.Schema{
						Type:        genai.TypeString,
						Description: "Action identifier that will be processed by the frontend. Common actions: refresh_schema etc.",
					},
					"isPrimary": &genai.Schema{
						Type:        genai.TypeBoolean,
						Description: "Whether this is a primary (highlighted) action button.",
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
			Type:        genai.TypeArray,
			Description: "An array of queries that the AI has generated. Return queries only when it makes sense to return a query, otherwise return empty array.",
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
						Required: []string{"paginatedQuery", "countQuery"},
						Properties: map[string]*genai.Schema{
							"paginatedQuery": &genai.Schema{
								Type: genai.TypeString,
							},
							"countQuery": &genai.Schema{
								Type:        genai.TypeString,
								Description: "(Only applicable for Fetching, Getting data) A fetch count query to get the total count of the original query, this query will not fetch original query data but only fetch count of the original query from the DB so that we can use the total count for pagination",
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
						Type:        genai.TypeString,
						Description: "MUST BE VALID JSON STRING with no additional text. [{\"column1\":\"value1\",\"column2\":\"value2\"}] or {\"result\":\"1 row affected\"}. Avoid giving too much data in the exampleResultString, just give 1-2 rows of data or if there is too much data, then give only limited fields of data, if a field contains too much data, then give less data from that field",
					},
				},
			},
		},
		"actionButtons": &genai.Schema{
			Type:        genai.TypeArray,
			Description: "List of action buttons to display to the user. Use these to suggest helpful actions like refreshing schema when schema issues are detected.",
			Items: &genai.Schema{
				Type:     genai.TypeObject,
				Enum:     []string{},
				Required: []string{"label", "action", "isPrimary"},
				Properties: map[string]*genai.Schema{
					"label": &genai.Schema{
						Type:        genai.TypeString,
						Description: "Display text for the button that the user will see.",
					},
					"action": &genai.Schema{
						Type:        genai.TypeString,
						Description: "Action identifier that will be processed by the frontend. Common actions: refresh_schema etc.",
					},
					"isPrimary": &genai.Schema{
						Type:        genai.TypeBoolean,
						Description: "Whether this is a primary (highlighted) action button.",
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
			Type:        genai.TypeArray,
			Description: "An array of queries that the AI has generated. Return queries only when it makes sense to return a query, otherwise return empty array.",
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
						Required: []string{"paginatedQuery", "countQuery"},
						Properties: map[string]*genai.Schema{
							"paginatedQuery": &genai.Schema{
								Type: genai.TypeString,
							},
							"countQuery": &genai.Schema{
								Type:        genai.TypeString,
								Description: "(Only applicable for Fetching, Getting data) A fetch count query to get the total count of the original query, this query will not fetch original query data but only fetch count of the original query from the DB so that we can use the total count for pagination",
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
						Type:        genai.TypeString,
						Description: "MUST BE VALID JSON STRING with no additional text. [{\"column1\":\"value1\",\"column2\":\"value2\"}] or {\"result\":\"1 row affected\"}. Avoid giving too much data in the exampleResultString, just give 1-2 rows of data or if there is too much data, then give only limited fields of data, if a field contains too much data, then give less data from that field",
					},
				},
			},
		},
		"actionButtons": &genai.Schema{
			Type:        genai.TypeArray,
			Description: "List of action buttons to display to the user. Use these to suggest helpful actions like refreshing schema when schema issues are detected.",
			Items: &genai.Schema{
				Type:     genai.TypeObject,
				Enum:     []string{},
				Required: []string{"label", "action", "isPrimary"},
				Properties: map[string]*genai.Schema{
					"label": &genai.Schema{
						Type:        genai.TypeString,
						Description: "Display text for the button that the user will see.",
					},
					"action": &genai.Schema{
						Type:        genai.TypeString,
						Description: "Action identifier that will be processed by the frontend. Common actions: refresh_schema etc.",
					},
					"isPrimary": &genai.Schema{
						Type:        genai.TypeBoolean,
						Description: "Whether this is a primary (highlighted) action button.",
					},
				},
			},
		},
		"assistantMessage": &genai.Schema{
			Type: genai.TypeString,
		},
	},
}

var GeminiMongoDBLLMResponseSchema = &genai.Schema{
	Type:     genai.TypeObject,
	Enum:     []string{},
	Required: []string{"assistantMessage"},
	Properties: map[string]*genai.Schema{
		"queries": &genai.Schema{
			Type:        genai.TypeArray,
			Description: "An array of queries that the AI has generated. Return queries only when it makes sense to return a query, otherwise return empty array.",
			Items: &genai.Schema{
				Type:     genai.TypeObject,
				Enum:     []string{},
				Required: []string{"query", "queryType", "isCritical", "canRollback", "explanation", "estimateResponseTime", "pagination", "exampleResultString"},
				Properties: map[string]*genai.Schema{
					"query": &genai.Schema{
						Type: genai.TypeString,
					},
					"collections": &genai.Schema{
						Type: genai.TypeString,
					},
					"queryType": &genai.Schema{
						Type: genai.TypeString,
					},
					"pagination": &genai.Schema{
						Type:     genai.TypeObject,
						Enum:     []string{},
						Required: []string{"paginatedQuery", "countQuery"},
						Properties: map[string]*genai.Schema{
							"paginatedQuery": &genai.Schema{
								Type: genai.TypeString,
							},
							"countQuery": &genai.Schema{
								Type:        genai.TypeString,
								Description: "(Only applicable for Fetching, Getting data) A fetch count query to get the total count of the original query, this query will not fetch original query data but only fetch count of the original query from the DB so that we can use the total count for pagination",
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
						Type:        genai.TypeString,
						Description: "MUST BE VALID JSON STRING with no additional text. [{\"column1\":\"value1\",\"column2\":\"value2\"}] or {\"result\":\"1 row affected\"}. Avoid giving too much data in the exampleResultString, just give 1-2 rows of data or if there is too much data, then give only limited fields of data, if a field contains too much data, then give less data from that field",
					},
					"validationSchema": &genai.Schema{
						Type: genai.TypeString,
					},
					"indexOptions": &genai.Schema{
						Type: genai.TypeString,
					},
				},
			},
		},
		"actionButtons": &genai.Schema{
			Type:        genai.TypeArray,
			Description: "List of action buttons to display to the user. Use these to suggest helpful actions like refreshing schema when schema issues are detected.",
			Items: &genai.Schema{
				Type:     genai.TypeObject,
				Enum:     []string{},
				Required: []string{"label", "action", "isPrimary"},
				Properties: map[string]*genai.Schema{
					"label": &genai.Schema{
						Type:        genai.TypeString,
						Description: "Display text for the button that the user will see.",
					},
					"action": &genai.Schema{
						Type:        genai.TypeString,
						Description: "Action identifier that will be processed by the frontend. Common actions: refresh_schema etc.",
					},
					"isPrimary": &genai.Schema{
						Type:        genai.TypeBoolean,
						Description: "Whether this is a primary (highlighted) action button.",
					},
				},
			},
		},
		"assistantMessage": &genai.Schema{
			Type: genai.TypeString,
		},
	},
}
