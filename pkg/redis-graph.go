package main

import (
	"strconv"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/data"
)

/**
 *  Represents node
 */
type nodeEntry struct {
	id       string
	title    string
	subTitle string
	mainStat string
	arc      int64
}

/**
 *  Represents edge
 */
type edgeEntry struct {
	id       string
	source   string
	target   string
	mainStat string
}

/**
 * GRAPH.QUERY <Graph name> {query}
 *
 * Executes the given query against a specified graph.
 * @see https://oss.redislabs.com/redisgraph/commands/#graphquery
 */
func queryGraphQuery(qm queryModel, client redisClient) backend.DataResponse {
	response := backend.DataResponse{}

	var result []interface{}

	// Run command
	err := client.RunFlatCmd(&result, "GRAPH.QUERY", qm.Key, qm.Cypher)

	// Check error
	if err != nil {
		return errorHandler(response, err)
	}

	// New Frame for nodes
	frameWithNodes := data.NewFrame("nodes")
	frameWithNodes.Meta = &data.FrameMeta{
		PreferredVisualization: "nodeGraph",
	}
	frameWithNodes.Fields = append(frameWithNodes.Fields, data.NewField("id", nil, []string{}))
	frameWithNodes.Fields = append(frameWithNodes.Fields, data.NewField("title", nil, []string{}))
	frameWithNodes.Fields = append(frameWithNodes.Fields, data.NewField("subTitle", nil, []string{}))
	frameWithNodes.Fields = append(frameWithNodes.Fields, data.NewField("mainStat", nil, []string{}))
	frameWithNodes.Fields = append(frameWithNodes.Fields, data.NewField("arc__", nil, []int64{}))

	// New Frame for edges
	frameWithEdges := data.NewFrame("edges")
	frameWithEdges.Meta = &data.FrameMeta{
		PreferredVisualization: "nodeGraph",
	}
	frameWithEdges.Fields = append(frameWithEdges.Fields, data.NewField("id", nil, []string{}))
	frameWithEdges.Fields = append(frameWithEdges.Fields, data.NewField("source", nil, []string{}))
	frameWithEdges.Fields = append(frameWithEdges.Fields, data.NewField("target", nil, []string{}))
	frameWithEdges.Fields = append(frameWithEdges.Fields, data.NewField("mainStat", nil, []string{}))

	// Adding frames to response
	response.Frames = append(response.Frames, frameWithNodes)
	response.Frames = append(response.Frames, frameWithEdges)

	existingNodes := map[string]bool{}

	for _, entries := range result[1].([]interface{}) {
		nodes, edges := findAllNodesAndEdges(entries)
		for _, node := range nodes {
			// Add each nodeEntry only once
			if _, ok := existingNodes[node.id]; !ok {
				frameWithNodes.AppendRow(node.id, node.title, node.subTitle, node.mainStat, node.arc)
				existingNodes[node.id] = true
			}
		}
		for _, edge := range edges {
			frameWithEdges.AppendRow(edge.id, edge.source, edge.target, edge.mainStat)
		}
	}
	return response
}

/**
 * Parse array of entries and find
 * either Nodes https://oss.redislabs.com/redisgraph/result_structure/#nodes
 * or Relations https://oss.redislabs.com/redisgraph/result_structure/#relations
 * and create corresponding nodeEntry or edgeEntry
 **/
func findAllNodesAndEdges(input interface{}) ([]nodeEntry, []edgeEntry) {
	nodes := []nodeEntry{}
	edges := []edgeEntry{}

	if entries, ok := input.([]interface{}); ok {
		for _, entry := range entries {
			entryFields := entry.([]interface{})

			// Node https://oss.redislabs.com/redisgraph/result_structure/#nodes
			if len(entryFields) == 3 {
				node := nodeEntry{arc: 1}
				idArray := entryFields[0].([]interface{})
				node.id = strconv.FormatInt(idArray[1].(int64), 10)

				// Assume first label will be a title if exists
				labelsArray := entryFields[1].([]interface{})
				labels := labelsArray[1].([]interface{})
				if len(labels) > 0 {
					node.title = string(labels[0].([]byte))
				}

				// Assume first property will be a mainStat if exists
				propertiesArray := entryFields[2].([]interface{})
				properties := propertiesArray[1].([]interface{})
				if len(properties) > 0 {
					propertyArray := properties[0].([]interface{})
					switch propValue := propertyArray[1].(type) {
					case []byte:
						node.mainStat = string(propValue)
					case int64:
						node.mainStat = strconv.FormatInt(propValue, 10)
					}
				}

				nodes = append(nodes, node)
			}

			// Relation https://oss.redislabs.com/redisgraph/result_structure/#relations
			if len(entryFields) == 5 {
				edge := edgeEntry{}
				idArray := entryFields[0].([]interface{})
				edge.id = strconv.FormatInt(idArray[1].(int64), 10)

				// Main Stat
				typeArray := entryFields[1].([]interface{})
				edge.mainStat = string(typeArray[1].([]byte))

				// Source
				srcArray := entryFields[2].([]interface{})
				edge.source = strconv.FormatInt(srcArray[1].(int64), 10)

				// Target
				destArray := entryFields[3].([]interface{})
				edge.target = strconv.FormatInt(destArray[1].(int64), 10)

				edges = append(edges, edge)
			}
		}
	}
	return nodes, edges
}

/**
 * GRAPH.SLOWLOG <Graph name>
 *
 * Returns a list containing up to 10 of the slowest queries issued against the given graph ID.
 * @see https://oss.redislabs.com/redisgraph/commands/#graphslowlog
 */
func queryGraphSlowlog(qm queryModel, client redisClient) backend.DataResponse {
	response := backend.DataResponse{}

	var result [][]string

	// Run command
	err := client.RunFlatCmd(&result, "GRAPH.SLOWLOG", qm.Key)

	// Check error
	if err != nil {
		return errorHandler(response, err)
	}

	// New Frame
	frame := data.NewFrame("GRAPH.SLOWLOG")
	frame.Fields = append(frame.Fields, data.NewField("timestamp", nil, []time.Time{}))
	frame.Fields = append(frame.Fields, data.NewField("command", nil, []string{}))
	frame.Fields = append(frame.Fields, data.NewField("query", nil, []string{}))
	frame.Fields = append(frame.Fields, data.NewField("duration", nil, []float64{}))
	response.Frames = append(response.Frames, frame)

	// Set Field Config
	frame.Fields[3].Config = &data.FieldConfig{Unit: "µs"}

	// Entries
	for _, entry := range result {
		timestamp, _ := strconv.ParseInt(entry[0], 10, 64)
		duration, _ := strconv.ParseFloat(entry[3], 64)
		frame.AppendRow(time.Unix(timestamp, 0), entry[1], entry[2], duration)
	}

	// Return
	return response
}
