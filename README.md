# Event Reader Service

This service is a sibling of the [event-store](https://github.com/tobyjsullivan/event-store)
service and offers the query component of the CQRS implementation.

## Getting Started

### AWS Configuration

The service reads events from a DynamoDB table. In order to do so, you
need to configure IAM credentials.

1. Create an AWS key/secret pair with the following permissions on the
DynamoDB table containing your events (see `event-store` readme for
details).
  - Query
2. Set the following environment variables.
  - AWS_ACCESS_KEY_ID
  - AWS_SECRET_ACCESS_KEY
  - AWS_REGION
  - DYNAMODB_TABLE

### Running with Docker

1. Build the Docker image
  - `docker build -t event-reader .`
2. Create a `.env` file with the following vars
  - `AWS_ACCESS_KEY_ID`
  - `AWS_SECRET_ACCESS_KEY`
  - `AWS_REGION`
  - `DYNAMODB_TABLE`
  - `PORT` (Optional. Default: 3000)
3. Run the docker container
  - `docker run -p 3000:3000 --env-file './.env' event-reader`

### Running with Gin

Gin allows live reload during development.

1. Install gin
  - `go get github.com/codegangsta/gin`
2. Set all required env vars
3. Run with gin (ensure your pwd is this cloned repo)
  - `gin`
4. Test the server
  - `curl http://127.0.0.1:3000`

## API

The service exposes a RESTful API for reading events.

### GET /:streamId

This request will return a paginated list of events for the specified
stream.

#### Optional query parameters

- `offset` The version from which to start the query
- `limit` The maximum number of events to return. The application may
silently enforce a stricter limit as necessary so never assume a page
size.

#### Response format

A successful request returns a JSON payload with the following format.

```json
{
  "pagination": {
    "nextOffset": 3
  },
  "payload": [
    {
      "streamId": "my-account",
      "version": 0,
      "type": "AccountCreated",
      "data": "eyJmaXJzdE5hbWUiOiJFeGFtcGxlIiwibGFzdE5hbWUiOiJDdXN0b21lciJ9"
    },
    {
      "streamId": "my-account",
      "version": 1,
      "type": "AmountDeposited",
      "data": "eyJhbW91bnQiOjEwMDAwLCJ0aW1lc3RhbXAiOiIyMDE3LTA1LTIwVDIyOjMwOjI2WiJ9"
    },
    {
      "streamId": "my-account",
      "version": 2,
      "type": "AmountWithdrawn",
      "data": "eyJhbW91bnQiOjQ2MCwidGltZXN0YW1wIjoiMjAxNy0wNS0yMVQxNDoyMDo1N1oifQ=="
    }
  ]
}
```

The `nextOffset` value may point to a version which does not yet exist.
In this case, specifying this offset in the query parameters will result
in an empty list in the response and the same `nextOffset` value. This
enables simple continuous polling for new events.

It is perfectly acceptable to request events from a stream which does
not yet exist. This will result in a `200 OK` response with an empty
results list.

#### Response codes

- 200 OK
- 400 BAD_REQUEST

