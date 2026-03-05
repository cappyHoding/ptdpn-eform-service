package vida

import (
	"encoding/json"
	"time"
)

// pollInterval adalah base interval antar polling.
// Setiap service mengalikan ini sesuai kebutuhannya.
const pollInterval = time.Second

// parseJSONStatus helper untuk unmarshal response saat polling.
// Digunakan oleh isDone callback di PollUntilDone.
func parseJSONStatus(body []byte, v interface{}) error {
	return json.Unmarshal(body, v)
}
