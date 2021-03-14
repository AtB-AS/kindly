# kindly
Utility library and tools for working with the Kindly.ai API

## CSV Frontend
Serves CSV from the kindly.ai Statistics API for consumption in Power BI.

### Endpoints
* `/labels`: Triggered chat labels.
* `/messages`: User messages.
* `/sessions`: User sessions.

#### Query parameters:
* `limit`: max number of rows to return (default: 10)
* `from`: from date (format: `2006-01-02`, default: `now - 24 hours`)
* `to`: to date (format: `2006-01-02`, default: `now`)
