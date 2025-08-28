package retry

import (
	"errors"
	"net"

	"github.com/lib/pq"
)

func IsRetryableErrorDatabase(err error) bool {
	if err == nil {
		return false
	}
	var pqErr *pq.Error
	if errors.As(err, &pqErr) {
		switch pqErr.Code {
		case "40001", // serialization_failure
			"40P01", // deadlock_detected
			"53300", // too_many_connections
			"57P03", // cannot_connect_now
			"08006", // connection_failure
			"08001", // sqlclient_unable_to_establish_sqlconnection
			"08003", // connection_does_not_exist
			"08000", // connection_exception
			"08004", // sqlserver_rejected_establishment_of_sqlconnection
			"08007": // transaction_resolution_unknown
			return true
		}
	}

	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}
