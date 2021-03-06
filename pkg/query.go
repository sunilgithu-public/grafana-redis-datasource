package main

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
	"github.com/mediocregopher/radix/v3/resp/resp2"
)

/**
 * Query commands
 */
func query(ctx context.Context, query backend.DataQuery, client redisClient) backend.DataResponse {
	var qm queryModel

	// Unmarshal the json into our queryModel
	err := json.Unmarshal(query.JSON, &qm)
	log.DefaultLogger.Debug("QueryData", "JSON", query.JSON)

	// Error
	if err != nil {
		response := backend.DataResponse{}
		response.Error = err
		return response
	}

	// From and To
	from := query.TimeRange.From.UnixNano() / 1000000
	to := query.TimeRange.To.UnixNano() / 1000000

	// Handle Panic from any command
	defer func() {
		if err := recover(); err != nil {
			log.DefaultLogger.Error("PANIC", "command", err)
		}
	}()

	/**
	 * Custom Command using Query
	 */
	if qm.Query != "" {
		return queryCustomCommand(qm, client)
	}

	/**
	 * Supported commands
	 */
	switch qm.Command {
	/**
	 * Redis Timeseries
	 */
	case "ts.get":
		return queryTsGet(qm, client)
	case "ts.info":
		return queryTsInfo(qm, client)
	case "ts.queryindex":
		return queryTsQueryIndex(qm, client)
	case "ts.range":
		return queryTsRange(from, to, qm, client)
	case "ts.mrange":
		return queryTsMRange(from, to, qm, client)

	/**
	 * Hash, Set, etc.
	 */
	case "hgetall":
		return queryHGetAll(qm, client)
	case "hget":
		return queryHGet(qm, client)
	case "hmget":
		return queryHMGet(qm, client)
	case "smembers", "hkeys":
		return querySMembers(qm, client)
	case "type", "get", "ttl", "hlen", "xlen", "llen", "scard":
		return queryKeyCommand(qm, client)

	/**
	 * Info
	 */
	case "info":
		return queryInfo(qm, client)
	case "clientList":
		return queryClientList(qm, client)
	case "slowlogGet":
		return querySlowlogGet(qm, client)

	/**
	 * Streams
	 */
	case "xinfoStream":
		return queryXInfoStream(qm, client)
	case "xrange":
		return queryXRange(qm, client)
	case "xrevrange":
		return queryXRevRange(qm, client)

	/**
	 * Cluster
	 */
	case "clusterInfo":
		return queryClusterInfo(qm, client)
	case "clusterNodes":
		return queryClusterNodes(qm, client)

	/**
	 * RediSearch
	 */
	case "ft.info":
		return queryFtInfo(qm, client)

	/**
	 * Custom commands
	 */
	case "tmscan":
		return queryTMScan(qm, client)

	/**
	 * Redis Gears
	 */
	case "rg.pystats":
		return queryRgPystats(qm, client)
	case "rg.dumpregistrations":
		return queryRgDumpregistrations(qm, client)
	case "rg.pyexecute":
		return queryRgPyexecute(qm, client)

	/**
	 * Redis Graph
	 */
	case "graph.query":
		return queryGraphQuery(qm, client)
	case "graph.slowlog":
		return queryGraphSlowlog(qm, client)

	/**
	 * Default
	 */
	default:
		response := backend.DataResponse{}
		log.DefaultLogger.Error("Query", "Command", qm.Command)
		return response
	}
}

/**
 * Error Handler
 */
func errorHandler(response backend.DataResponse, err error) backend.DataResponse {
	var redisErr resp2.Error

	// Check for RESP2 Error
	if errors.As(err, &redisErr) {
		response.Error = redisErr.E
	} else {
		response.Error = err
	}

	// Return Response
	return response
}

/**
 * Commands with one key parameter and return value
 *
 * @see https://redis.io/commands/type
 * @see https://redis.io/commands/ttl
 * @see https://redis.io/commands/hlen
 */
func queryKeyCommand(qm queryModel, client redisClient) backend.DataResponse {
	response := backend.DataResponse{}

	// Execute command
	var value string
	err := client.RunCmd(&value, qm.Command, qm.Key)

	// Check error
	if err != nil {
		return errorHandler(response, err)
	}

	// Add the frames to the response
	response.Frames = append(response.Frames, createFrameValue(qm.Key, value))

	// Return Response
	return response
}
