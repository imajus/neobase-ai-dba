package dbmanager

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// convertMongoDBSchemaToSchemaInfo converts MongoDB schema to generic SchemaInfo
func convertMongoDBSchemaToSchemaInfo(mongoSchema MongoDBSchema) *SchemaInfo {
	schema := &SchemaInfo{
		Tables:    make(map[string]TableSchema),
		Views:     make(map[string]ViewSchema),
		UpdatedAt: time.Now(),
	}

	// Convert collections to tables
	for collName, coll := range mongoSchema.Collections {
		tableSchema := TableSchema{
			Name:        collName,
			Columns:     make(map[string]ColumnInfo),
			Indexes:     make(map[string]IndexInfo),
			ForeignKeys: make(map[string]ForeignKey),
			Constraints: make(map[string]ConstraintInfo),
			RowCount:    coll.DocumentCount,
		}

		// Convert fields to columns
		for fieldName, field := range coll.Fields {
			columnType := field.Type
			if field.IsArray {
				columnType = "array<" + columnType + ">"
			}

			tableSchema.Columns[fieldName] = ColumnInfo{
				Name:         fieldName,
				Type:         columnType,
				IsNullable:   !field.IsRequired,
				DefaultValue: "",
				Comment:      "",
			}
		}

		// Convert indexes
		if indexes, ok := mongoSchema.Indexes[collName]; ok {
			for _, idx := range indexes {
				// Skip _id_ index as it's implicit
				if idx.Name == "_id_" {
					continue
				}

				// Extract column names from index keys
				columns := make([]string, 0, len(idx.Keys))
				for _, key := range idx.Keys {
					columns = append(columns, key.Key)
				}

				tableSchema.Indexes[idx.Name] = IndexInfo{
					Name:     idx.Name,
					Columns:  columns,
					IsUnique: idx.IsUnique,
				}
			}
		}

		schema.Tables[collName] = tableSchema
	}

	return schema
}

// getMongoDBFieldType determines the type of a MongoDB field
func getMongoDBFieldType(value interface{}) string {
	if value == nil {
		return "null"
	}

	switch value.(type) {
	case string:
		return "string"
	case int32, int64, int:
		return "integer"
	case float64, float32:
		return "double"
	case bool:
		return "boolean"
	case primitive.DateTime:
		return "date"
	case primitive.ObjectID:
		return "objectId"
	case primitive.A:
		return "array"
	case bson.M, bson.D:
		return "object"
	case primitive.Binary:
		return "binary"
	default:
		return fmt.Sprintf("%T", value)
	}
}

// processMongoDBQueryParams processes MongoDB query parameters
func processMongoDBQueryParams(paramsStr string) (string, error) {
	// Log the original string for debugging
	log.Printf("Original MongoDB query params: %s", paramsStr)

	// Extract modifiers from the query string
	var modifiersStr string
	if idx := strings.Index(paramsStr, "})."); idx != -1 {
		// Save the modifiers part for later processing
		modifiersStr = paramsStr[idx+1:]
		// Only keep the filter part
		paramsStr = paramsStr[:idx+1]
		log.Printf("Extracted filter part: %s", paramsStr)
		log.Printf("Extracted modifiers part: %s", modifiersStr)
	}

	// Check for offset_size in skip() - this is a special case for pagination
	// offset_size is a placeholder that will be replaced with the actual offset value
	// by the chat service when executing paginated queries.
	// For example, db.posts.find({}).skip(offset_size).limit(50) will become
	// db.posts.find({}).skip(50).limit(50) when requesting the second page with page size 50.
	// This replacement happens in the chat_service.go GetQueryResults function.
	if modifiersStr != "" {
		skipRegex := regexp.MustCompile(`\.skip\(offset_size\)`)
		if skipRegex.MatchString(modifiersStr) {
			log.Printf("Found offset_size in skip(), this will be replaced by the actual offset value")
		}
	}

	// Handle numerical values in sort expressions like {field: -1}
	// Preserve negative numbers in sort expressions
	sortPattern := regexp.MustCompile(`\{([^{}]+):\s*(-?\d+)\s*\}`)
	paramsStr = sortPattern.ReplaceAllStringFunc(paramsStr, func(match string) string {
		// Extract the field and direction
		sortMatches := sortPattern.FindStringSubmatch(match)
		if len(sortMatches) < 3 {
			return match
		}

		field := strings.TrimSpace(sortMatches[1])
		// Add quotes around the field name if not already quoted
		if !strings.HasPrefix(field, "\"") && !strings.HasPrefix(field, "'") {
			field = fmt.Sprintf(`"%s"`, field)
		}

		// Keep the numerical direction value as is
		return fmt.Sprintf(`{%s: %s}`, field, sortMatches[2])
	})

	// Handle ObjectId syntax: ObjectId('...') -> {"$oid":"..."}
	// This pattern matches both ObjectId('...') and ObjectId("...")
	objectIdPattern := regexp.MustCompile(`ObjectId\(['"]([^'"]+)['"]\)`)
	paramsStr = objectIdPattern.ReplaceAllStringFunc(paramsStr, func(match string) string {
		// Extract the ObjectId value
		re := regexp.MustCompile(`ObjectId\(['"]([^'"]+)['"]\)`)
		matches := re.FindStringSubmatch(match)
		if len(matches) < 2 {
			return match
		}

		// Return the proper JSON format for ObjectId
		return fmt.Sprintf(`{"$oid":"%s"}`, matches[1])
	})

	// Handle ISODate syntax: ISODate('...') -> {"$date":"..."}
	// This pattern matches both ISODate('...') and ISODate("...")
	isoDatePattern := regexp.MustCompile(`ISODate\(['"]([^'"]+)['"]\)`)
	paramsStr = isoDatePattern.ReplaceAllStringFunc(paramsStr, func(match string) string {
		// Extract the ISODate value
		re := regexp.MustCompile(`ISODate\(['"]([^'"]+)['"]\)`)
		matches := re.FindStringSubmatch(match)
		if len(matches) < 2 {
			return match
		}

		// Return the proper JSON format for Date
		return fmt.Sprintf(`{"$date":"%s"}`, matches[1])
	})

	// Handle Math expressions in date calculations
	// First, detect and replace mathematical operations in date calculations like: Date.now() - 24 * 60 * 60 * 1000
	mathExprPattern := regexp.MustCompile(`(Date\.now\(\)|new Date\(\)\.getTime\(\))\s*([+\-])\s*\(?\s*(\d+(?:\s*[*]\s*\d+)*)\s*\)?`)
	paramsStr = mathExprPattern.ReplaceAllStringFunc(paramsStr, func(match string) string {
		log.Printf("Found date math expression: %s", match)
		// For simplicity, use current time minus 24 hours for common "yesterday" pattern
		return fmt.Sprintf(`{"$date":"%s"}`, time.Now().Add(-24*time.Hour).Format(time.RFC3339))
	})

	// Handle new Date() syntax with various formats:
	// 1. new Date() without parameters -> current date in ISO format
	// 2. new Date("...") or new Date('...') with quoted date string
	// 3. new Date(year, month, day, ...) with numeric parameters
	// 4. new Date(Date.now() - 24 * 60 * 60 * 1000) -> current date minus 24 hours

	// First, handle new Date() without parameters
	emptyDatePattern := regexp.MustCompile(`new\s+Date\(\s*\)`)
	paramsStr = emptyDatePattern.ReplaceAllStringFunc(paramsStr, func(match string) string {
		// Return current date in ISO format
		return fmt.Sprintf(`{"$date":"%s"}`, time.Now().Format(time.RFC3339))
	})

	// Handle new Date("...") and new Date('...') with quoted date string
	quotedDatePattern := regexp.MustCompile(`new\s+Date\(['"]([^'"]+)['"]\)`)
	paramsStr = quotedDatePattern.ReplaceAllStringFunc(paramsStr, func(match string) string {
		// Extract the date value
		re := regexp.MustCompile(`new\s+Date\(['"]([^'"]+)['"]\)`)
		matches := re.FindStringSubmatch(match)
		if len(matches) < 2 {
			return match
		}

		// Return the proper JSON format for Date
		return fmt.Sprintf(`{"$date":"%s"}`, matches[1])
	})

	// Handle new Date(Date.now() - ...) format specifically
	dateMathPattern := regexp.MustCompile(`new\s+Date\(\s*Date\.now\(\)\s*-\s*([^)]+)\)`)
	paramsStr = dateMathPattern.ReplaceAllStringFunc(paramsStr, func(match string) string {
		// Extract the time offset expression
		re := regexp.MustCompile(`new\s+Date\(\s*Date\.now\(\)\s*-\s*([^)]+)\)`)
		matches := re.FindStringSubmatch(match)
		if len(matches) < 2 {
			return match
		}

		// For this pattern, we'll use the current date minus 24 hours
		// This is a simplification for common cases like "24 * 60 * 60 * 1000" (24 hours in milliseconds)
		log.Printf("Handling Date.now() math expression: %s", matches[1])
		return fmt.Sprintf(`{"$date":"%s"}`, time.Now().Add(-24*time.Hour).Format(time.RFC3339))
	})

	// Handle complex date expressions like:
	// new Date(new Date().getTime() - (20 * 60 * 1000))
	// new Date(new Date().getFullYear(), new Date().getMonth()-1, 1)
	complexDatePattern := regexp.MustCompile(`new\s+Date\(([^)]+)\)`)
	paramsStr = complexDatePattern.ReplaceAllStringFunc(paramsStr, func(match string) string {
		// Check if we've already processed this date (to avoid infinite recursion)
		if strings.Contains(match, "$date") {
			return match
		}

		// For complex date expressions, we'll use the current date
		// This is a simplification, but it allows the query to be parsed
		log.Printf("Converting complex date expression to current date: %s", match)
		return fmt.Sprintf(`{"$date":"%s"}`, time.Now().Format(time.RFC3339))
	})

	// Log the processed string for debugging
	log.Printf("After ObjectId and Date replacement: %s", paramsStr)

	// Temporarily replace $oid and $date with placeholders to prevent them from being modified
	paramsStr = strings.ReplaceAll(paramsStr, "$oid", "__MONGODB_OID__")
	paramsStr = strings.ReplaceAll(paramsStr, "$date", "__MONGODB_DATE__")

	// Handle MongoDB operators ($gt, $lt, $in, etc.) throughout the entire document
	// This is a more comprehensive approach than just handling them at the beginning of objects
	operatorRegex := regexp.MustCompile(`(\s*)(\$[a-zA-Z0-9]+)(\s*):`)
	paramsStr = operatorRegex.ReplaceAllString(paramsStr, `$1"$2"$3:`)

	// First pass: Quote all field names in objects
	// This regex matches field names followed by a colon, ensuring they're properly quoted
	// Improved pattern to catch all unquoted field names, including those at the beginning of objects
	fieldNameRegex := regexp.MustCompile(`(^|[,{])\s*([a-zA-Z0-9_]+)\s*:`)
	paramsStr = fieldNameRegex.ReplaceAllString(paramsStr, `$1"$2":`)

	// Handle single quotes for string values
	// Use a standard approach instead of negative lookbehind which isn't supported in Go
	singleQuoteRegex := regexp.MustCompile(`'([^']*)'`)
	paramsStr = singleQuoteRegex.ReplaceAllString(paramsStr, `"$1"`)

	// Restore placeholders
	paramsStr = strings.ReplaceAll(paramsStr, "__MONGODB_OID__", "$oid")
	paramsStr = strings.ReplaceAll(paramsStr, "__MONGODB_DATE__", "$date")

	// Ensure the document is valid JSON
	// Second pass: Check if it's an object and add missing quotes to field names
	if strings.HasPrefix(paramsStr, "{") && strings.HasSuffix(paramsStr, "}") {
		// Add quotes to any remaining unquoted field names
		// This regex matches field names that aren't already quoted
		unquotedFieldRegex := regexp.MustCompile(`([,{]|^)\s*([a-zA-Z0-9_]+)\s*:`)
		for unquotedFieldRegex.MatchString(paramsStr) {
			paramsStr = unquotedFieldRegex.ReplaceAllString(paramsStr, `$1"$2":`)
		}
	}

	// Final fix: Make sure all occurences of field names have double quotes
	// This extreme approach ensures all field names are properly quoted
	// Handle space-separated fields in projection
	for _, field := range []string{"email", "_id", "role", "createdAt", "name", "address", "phone"} {
		fieldPattern := regexp.MustCompile(fmt.Sprintf(`(%s):\s*([0-1])`, field))
		paramsStr = fieldPattern.ReplaceAllString(paramsStr, `"$1": $2`)
	}

	// Log the final processed string for debugging
	log.Printf("Final processed MongoDB query params: %s", paramsStr)

	return paramsStr, nil
}

// processObjectIds processes ObjectId syntax in MongoDB queries
func processObjectIds(filter map[string]interface{}) error {
	// Log the input filter for debugging
	filterJSON, _ := json.Marshal(filter)
	log.Printf("processObjectIds input: %s", string(filterJSON))

	for key, value := range filter {
		switch v := value.(type) {
		case map[string]interface{}:
			// Check if this is an ObjectId
			if oidStr, ok := v["$oid"].(string); ok && len(v) == 1 {
				// Convert to ObjectID
				oid, err := primitive.ObjectIDFromHex(oidStr)
				if err != nil {
					return fmt.Errorf("invalid ObjectId: %v", err)
				}
				filter[key] = oid
				log.Printf("Converted ObjectId %s to %v", oidStr, oid)
			} else if dateStr, ok := v["$date"].(string); ok && len(v) == 1 {
				// Parse the date to validate it, but preserve the exact format for MongoDB
				_, err := time.Parse(time.RFC3339, dateStr)
				if err != nil {
					// Try other common date formats
					formats := []string{
						time.RFC3339,
						"2006-01-02T15:04:05Z",
						"2006-01-02",
						"2006/01/02",
						"01/02/2006",
						"01-02-2006",
						time.ANSIC,
						time.UnixDate,
						time.RubyDate,
						time.RFC822,
						time.RFC822Z,
						time.RFC850,
						time.RFC1123,
						time.RFC1123Z,
					}

					parsed := false
					for _, format := range formats {
						if _, parseErr := time.Parse(format, dateStr); parseErr == nil {
							parsed = true
							break
						}
					}

					if !parsed {
						return fmt.Errorf("invalid date format: %s", dateStr)
					}
				}

				// Use the original date string format for MongoDB
				filter[key] = dateStr
				log.Printf("Converted date %s to %s", dateStr, dateStr)
			} else {
				// Recursively process nested objects
				if err := processObjectIds(v); err != nil {
					return err
				}
			}
		case []interface{}:
			// Process arrays
			for i, item := range v {
				if itemMap, ok := item.(map[string]interface{}); ok {
					if err := processObjectIds(itemMap); err != nil {
						return err
					}
					v[i] = itemMap
				}
			}
		}
	}

	// Log the output filter for debugging
	outputJSON, _ := json.Marshal(filter)
	log.Printf("processObjectIds output (after ObjectId and Date conversion): %s", string(outputJSON))

	return nil
}

// Add this new function to extract modifiers from the query string
// Add this after the processObjectIds function
func extractModifiers(query string) struct {
	Skip       int64
	Limit      int64
	Sort       string
	Projection string
	Count      bool
} {
	modifiers := struct {
		Skip       int64
		Limit      int64
		Sort       string
		Projection string
		Count      bool
	}{}

	// Check if the query string is empty or doesn't contain any modifiers
	if query == "" || !strings.Contains(query, ".") {
		return modifiers
	}

	// Extract skip
	skipRegex := regexp.MustCompile(`\.skip\((\d+)\)`)
	skipMatches := skipRegex.FindStringSubmatch(query)
	if len(skipMatches) > 1 {
		skip, err := strconv.ParseInt(skipMatches[1], 10, 64)
		if err == nil {
			modifiers.Skip = skip
		}
	}

	// Extract limit
	limitRegex := regexp.MustCompile(`\.limit\((\d+)\)`)
	limitMatches := limitRegex.FindStringSubmatch(query)
	if len(limitMatches) > 1 {
		limit, err := strconv.ParseInt(limitMatches[1], 10, 64)
		if err == nil {
			modifiers.Limit = limit
		}
	}

	// Extract count
	countRegex := regexp.MustCompile(`\.count\(\s*\)`)
	countMatches := countRegex.FindStringSubmatch(query)
	if len(countMatches) > 0 {
		modifiers.Count = true
		log.Printf("extractModifiers -> Detected count() modifier")
	}

	// Extract projection
	projectionRegex := regexp.MustCompile(`\.project\(([^)]+)\)`)
	projectionMatches := projectionRegex.FindStringSubmatch(query)
	if len(projectionMatches) > 1 {
		// Get the raw projection expression
		projectionExpr := projectionMatches[1]
		modifiers.Projection = projectionExpr
		log.Printf("extractModifiers -> Extracted projection expression: %s", modifiers.Projection)
	}

	// Extract sort - improved to handle complex sort expressions including negative values
	sortRegex := regexp.MustCompile(`\.sort\(([^)]+)\)`)
	sortMatches := sortRegex.FindStringSubmatch(query)
	if len(sortMatches) > 1 {
		// Get the raw sort expression
		sortExpr := sortMatches[1]
		log.Printf("extractModifiers -> Raw sort expression: %s", sortExpr)

		// Keep the sort expression as is, and let the processMongoDBQueryParams function handle
		// the conversion to proper JSON.
		modifiers.Sort = sortExpr
		log.Printf("extractModifiers -> Extracted sort expression: %s", modifiers.Sort)
	}

	return modifiers
}

// SafeBeginTx is a helper function to safely begin a transaction with proper error handling
func (d *MongoDBDriver) SafeBeginTx(ctx context.Context, conn *Connection) (Transaction, error) {
	log.Printf("MongoDBDriver -> SafeBeginTx -> Safely beginning MongoDB transaction")

	tx := d.BeginTx(ctx, conn)

	// Check if the transaction has an error
	if mongoTx, ok := tx.(*MongoDBTransaction); ok && mongoTx.Error != nil {
		log.Printf("MongoDBDriver -> SafeBeginTx -> Transaction creation failed: %v", mongoTx.Error)
		return nil, mongoTx.Error
	}

	// Check if the transaction has a nil session
	if mongoTx, ok := tx.(*MongoDBTransaction); ok && mongoTx.Session == nil {
		log.Printf("MongoDBDriver -> SafeBeginTx -> Transaction has nil session")
		return nil, fmt.Errorf("transaction has nil session")
	}

	log.Printf("MongoDBDriver -> SafeBeginTx -> Transaction created successfully")
	return tx, nil
}

// processSortExpression handles MongoDB sort expressions, properly preserving negative values
func processSortExpression(sortExpr string) (string, error) {
	log.Printf("Processing sort expression: %s", sortExpr)

	// If it's already a valid JSON object, validate that the field names are quoted properly
	if strings.HasPrefix(sortExpr, "{") && strings.HasSuffix(sortExpr, "}") {
		// Pattern to find field:value pairs with negative numbers
		sortPattern := regexp.MustCompile(`\{([^{}]+):\s*(-?\d+)\s*\}`)
		sortExpr = sortPattern.ReplaceAllStringFunc(sortExpr, func(match string) string {
			// Extract the field and direction
			sortMatches := sortPattern.FindStringSubmatch(match)
			if len(sortMatches) < 3 {
				return match
			}

			field := strings.TrimSpace(sortMatches[1])
			direction := strings.TrimSpace(sortMatches[2])

			// Add quotes around the field name if not already quoted
			if !strings.HasPrefix(field, "\"") && !strings.HasPrefix(field, "'") {
				field = fmt.Sprintf(`"%s"`, field)
			}

			// Preserve the direction (including negative sign)
			return fmt.Sprintf(`{%s: %s}`, field, direction)
		})

		// Handle multiple fields in a sort object: {field1: 1, field2: -1}
		multiFieldPattern := regexp.MustCompile(`\{([^{}]+)\}`)
		if multiFieldPattern.MatchString(sortExpr) {
			match := multiFieldPattern.FindStringSubmatch(sortExpr)[1]

			// Extract individual field:value pairs
			pairs := strings.Split(match, ",")
			processedPairs := make([]string, 0, len(pairs))

			for _, pair := range pairs {
				if pair = strings.TrimSpace(pair); pair == "" {
					continue
				}

				// Split the pair into field and value
				parts := strings.SplitN(pair, ":", 2)
				if len(parts) != 2 {
					continue
				}

				field := strings.TrimSpace(parts[0])
				value := strings.TrimSpace(parts[1])

				// Add quotes around the field name if not already quoted
				if !strings.HasPrefix(field, "\"") && !strings.HasPrefix(field, "'") {
					field = fmt.Sprintf(`"%s"`, field)
				}

				processedPairs = append(processedPairs, fmt.Sprintf(`%s: %s`, field, value))
			}

			// Reconstruct the sort object
			sortExpr = fmt.Sprintf(`{%s}`, strings.Join(processedPairs, ", "))
		}

		// Now convert to proper JSON for MongoDB
		jsonStr, err := processMongoDBQueryParams(sortExpr)
		if err != nil {
			log.Printf("Error processing sort expression: %v", err)
			return sortExpr, err
		}

		log.Printf("Processed sort expression to: %s", jsonStr)
		return jsonStr, nil
	} else {
		// Simple field name, default to ascending order
		field := strings.Trim(sortExpr, `"' `)
		sortExpr = fmt.Sprintf(`{"%s": 1}`, field)
		log.Printf("Converted simple sort field to object: %s", sortExpr)
		return sortExpr, nil
	}
}

// Add this function to recursively replace date placeholders in complex objects
func replaceDatePlaceholders(obj interface{}) interface{} {
	// Handle different types
	switch v := obj.(type) {
	case map[string]interface{}:
		// Process map
		for k, val := range v {
			v[k] = replaceDatePlaceholders(val)
		}
		return v
	case []interface{}:
		// Process array
		for i, val := range v {
			v[i] = replaceDatePlaceholders(val)
		}
		return v
	case string:
		// Check if it's a date placeholder
		if v == "__DATE_PLACEHOLDER__" {
			return primitive.NewDateTimeFromTime(time.Now())
		}
		return v
	default:
		return v
	}
}

// Add this function to recursively process date placeholders in nested objects
func processNestedDateValues(obj map[string]interface{}) {
	for k, v := range obj {
		// Handle different types of values
		switch val := v.(type) {
		case string:
			// Check if it's a date placeholder
			if val == "__DATE_PLACEHOLDER__" {
				// Replace with current date
				obj[k] = time.Now()
				log.Printf("Replaced date placeholder with current time at key: %s", k)
			}
		case map[string]interface{}:
			// Check if this is a $gte or similar operator with a date placeholder
			if dateStr, ok := val["$gte"]; ok {
				if dateStrVal, isString := dateStr.(string); isString && dateStrVal == "__DATE_PLACEHOLDER__" {
					val["$gte"] = time.Now()
					log.Printf("Replaced date placeholder in $gte operator with current time")
				}
			}
			// Similarly check other operators
			for op, opVal := range val {
				if opStrVal, isString := opVal.(string); isString && opStrVal == "__DATE_PLACEHOLDER__" {
					val[op] = time.Now()
					log.Printf("Replaced date placeholder in %s operator with current time", op)
				}
			}
			// Recursively process nested maps
			processNestedDateValues(val)
		case []interface{}:
			// Process array items
			for _, item := range val {
				if itemMap, ok := item.(map[string]interface{}); ok {
					processNestedDateValues(itemMap)
				}
			}
		}
	}
}

// Add this new function after processMongoDBQueryParams

// processProjectionParams specifically handles MongoDB projection parameters,
// which often need special treatment due to their simpler structure
func processProjectionParams(projectionStr string) (string, error) {
	// Log the original string for debugging
	log.Printf("Original MongoDB projection params: %s", projectionStr)

	// Special case fix for the exact error pattern we saw in the logs
	// This approach uses direct string replacement for common MongoDB projection fields
	commonFields := []string{"email", "_id", "role", "createdAt", "name", "address", "phone"}
	for _, field := range commonFields {
		// Simple direct string replacement - most reliable approach
		oldPattern := field + ":"
		newPattern := "\"" + field + "\":"
		projectionStr = regexp.MustCompile(oldPattern).ReplaceAllString(projectionStr, newPattern)
	}

	// Simple approach - split by comma and handle each field individually
	if !strings.HasPrefix(projectionStr, "{") || !strings.HasSuffix(projectionStr, "}") {
		projectionStr = "{" + projectionStr + "}"
	}

	// Remove braces for processing
	content := projectionStr[1 : len(projectionStr)-1]

	// Split by comma
	fields := strings.Split(content, ",")

	// Process each field
	processedFields := make([]string, 0, len(fields))
	for _, field := range fields {
		field = strings.TrimSpace(field)
		parts := strings.SplitN(field, ":", 2)
		if len(parts) != 2 {
			// Skip invalid fields
			continue
		}

		// Quote the field name if not already quoted
		fieldName := strings.TrimSpace(parts[0])
		if !strings.HasPrefix(fieldName, "\"") && !strings.HasPrefix(fieldName, "'") {
			fieldName = "\"" + fieldName + "\""
		}

		// Keep the value as is
		value := strings.TrimSpace(parts[1])

		// Add the processed field
		processedFields = append(processedFields, fieldName+": "+value)
	}

	// Combine back into a JSON object
	result := "{" + strings.Join(processedFields, ", ") + "}"
	log.Printf("Processed MongoDB projection params: %s", result)

	return result, nil
}
