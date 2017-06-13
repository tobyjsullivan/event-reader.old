package main

import (
	"os"

	"fmt"
	"net/http"

	"github.com/urfave/negroni"
	"github.com/gorilla/mux"
	"log"
	"strconv"
    "github.com/aws/aws-sdk-go/service/dynamodb"
    "github.com/aws/aws-sdk-go/aws/session"
    "github.com/aws/aws-sdk-go/aws"
    "github.com/aws/aws-sdk-go/aws/credentials"
    "encoding/json"
)

const (
    STARTING_VERSION = 1
    DEFAULT_LIMIT = 20
)

var (
	logger *log.Logger
    svc *dynamodb.DynamoDB
    dynamoTable string
)

func init() {
	logger = log.New(os.Stdout, "[event-reader] ", 0)

    require("AWS_ACCESS_KEY_ID")
    require("AWS_SECRET_ACCESS_KEY")

    sess := session.Must(session.NewSession(&aws.Config{
        Credentials: credentials.NewEnvCredentials(),
        Region: aws.String(require("AWS_REGION")),
    }))
    svc = dynamodb.New(sess)

    dynamoTable = require("DYNAMODB_TABLE")
}

func require(envvar string) string {
    v := os.Getenv(envvar)
    if v == "" {
        panic(fmt.Sprintf("Must set %s", envvar))
    }

    return v
}

func main() {
	r := buildRoutes()

	n := negroni.New()
	n.UseHandler(r)

	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}

	n.Run(":" + port)
}

func buildRoutes() http.Handler {
	r := mux.NewRouter()
	r.HandleFunc("/", statusHandler).Methods("GET")
	r.HandleFunc("/{streamId}", readStreamHandler).Methods("GET")

	return r
}

func statusHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "The reader service is online.\n")
}

func readStreamHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	streamId := vars["streamId"]
	if streamId == "" {
		logger.Println("Invalid stream ID")
		http.Error(w, "Invalid Stream ID", http.StatusBadRequest)
		return
	}
	logger.Println("Stream ID:", streamId)

	q := r.URL.Query()
    var err error
	limit := DEFAULT_LIMIT
	if v := q.Get("limit"); v != "" {
		limit, err = strconv.Atoi(v)
		if err != nil {
			logger.Println("Error parsing limit:", err.Error())
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}

	offset := STARTING_VERSION
	if v := q.Get("offset"); v != "" {
		offset, err = strconv.Atoi(v)
		if err != nil {
			logger.Println("Error parsing offset:", err.Error())
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}

    events, err := fetchEvents(streamId, offset, limit)
    if err != nil {
        logger.Println("Error fetching events:", err.Error())
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    nextOffset := STARTING_VERSION
    if len(events) > 0 {
        nextOffset = events[len(events) - 1].Version + 1
    }

    response := &readStreamResponse{
        Pagination: &pagination{
           NextOffset: nextOffset,
        },
        Payload: events,
    }
    encoder := json.NewEncoder(w)
    if err = encoder.Encode(response); err != nil {
        logger.Println("Error encoding response:", err.Error())
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
}

type readStreamResponse struct {
    Pagination *pagination `json:"pagination"`
    Payload []*event `json:"payload"`
}

type pagination struct {
    NextOffset int `json:"nextOffset"`
}

type event struct {
	StreamId string `json:"streamId"`
    Version int `json:"version"`
    Type string `json:"type"`
    Data string `json:"data"`
}

func fetchEvents(streamId string, offset, limit int) ([]*event, error) {
    logger.Println("Querying DynamoDB.")
    res, err := svc.Query(&dynamodb.QueryInput{
        TableName:                 aws.String(dynamoTable),
        ConsistentRead:            aws.Bool(false),
        ExpressionAttributeNames:  map[string]*string{
            "#entityColumn": aws.String("Entity ID"),
            "#version": aws.String("Version"),
            "#eventType": aws.String("Event Type"),
            "#data": aws.String("Data"),
        },
        ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
            ":entityId": {S: aws.String(streamId)},
            ":offset": {N: aws.String(strconv.Itoa(offset))},
        },
        ProjectionExpression:      aws.String("#version, #eventType, #data"),
        KeyConditionExpression:    aws.String("#entityColumn = :entityId and #version >= :offset"),
        Limit:            aws.Int64(int64(limit)),
        ScanIndexForward: aws.Bool(true),
    })
    if err != nil {
        logger.Println("DynamoDB query failed.", err.Error())
        return []*event{}, err
    }

    out := make([]*event, aws.Int64Value(res.Count))
    for i, item := range res.Items {
        v := item["Version"]
        if v == nil || v.N == nil || aws.StringValue(v.N) == "" {
            logger.Println("Couldn't find version in received event", item)
        }

        version, err := strconv.Atoi(aws.StringValue(v.N))
        if err != nil {
            logger.Println("Error parsing version number of event:", err.Error())
        }

        out[i] = &event{
            StreamId: streamId,
            Version: version,
            Type: aws.StringValue(item["Event Type"].S),
            Data: aws.StringValue(item["Data"].S),
        }
    }

    return out, nil
}


