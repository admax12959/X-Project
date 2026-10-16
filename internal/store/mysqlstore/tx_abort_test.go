package mysqlstore

import (
	"fmt"
	"testing"

	"github.com/go-sql-driver/mysql"
)

func TestIsTxAbortingKnowsTheErrorsThatEndATransaction(t *testing.T) {
	s := &Store{}
	for num, want := range map[uint16]bool{1213: true, 1205: true, 1062: false, 1364: false, 1048: false} {
		err := fmt.Errorf("insert: %w", &mysql.MySQLError{Number: num})
		if got := s.IsTxAborting(err); got != want {
			t.Errorf("error %d: IsTxAborting = %v, want %v", num, got, want)
		}
	}
	if !s.IsTxAborting(mysql.ErrInvalidConn) {
		t.Error("an invalid connection must end the transaction")
	}
}
