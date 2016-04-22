package dbms

import (
	"database/sql"
	"fmt"
	_ "github.com/denisenkom/go-mssqldb"
	"log"
)

var (
	password = "Qwe`123"
	port     = 1433
	user     = "sa"
)

func check_trace_on_1224() string {

	Result := "is19-p-db-10 (нода №1): " + check_trace_on_1224_on_server("10.126.200.34") + "\n" +
		"is19-p-db-11 (нода №0): " + check_trace_on_1224_on_server("10.126.200.35") + "\n" +
		"is19-p-db-14 (нода №2): " + check_trace_on_1224_on_server("10.126.200.69") + "\n" +
		"is19-p-db-15 (нода №3): " + check_trace_on_1224_on_server("10.126.200.70") + "\n" +
		"is19-p-db-16 (нода №4): " + check_trace_on_1224_on_server("10.126.200.89") + "\n" +
		"is19-p-db-17 (нода №0): " + check_trace_on_1224_on_server("172.16.241.26") + "\n" +
		"is19-p-db-23 (нода №4): " + check_trace_on_1224_on_server("172.16.241.35") + "\n" +
		"is19-p-db-25 (нода №9): " + check_trace_on_1224_on_server("172.16.241.36") + "\n" +
		"is19-p-db-27 (нода №5): " + check_trace_on_1224_on_server("172.16.241.37") + "\n" +
		"is19-p-db-29 (нода №6): " + check_trace_on_1224_on_server("172.16.241.38") + "\n" +
		"is19-p-db-31 (нода №3): " + check_trace_on_1224_on_server("172.16.241.39") + "\n" +
		"is19-p-db-33 (нода №7): " + check_trace_on_1224_on_server("172.16.241.109") + "\n" +
		"is19-p-db-34 (нода №8): " + check_trace_on_1224_on_server("172.16.241.115") + "\n" +
		"is19-p-db-35 (нода №10): " + check_trace_on_1224_on_server("172.16.241.126") + "\n" +
		"is19-p-db-36 (нода №11): " + check_trace_on_1224_on_server("172.16.241.132") + "\n"
	return Result
}

func check_trace_on_1224_on_server(server string) string {

	connString := fmt.Sprintf("server=%s;user id=%s;password=%s;port=%d;encrypt=disable", server, user, password, port)

	conn, err := sql.Open("mssql", connString)
	if err != nil {
		log.Fatal("Open connection failed:", err.Error())
	}
	defer conn.Close()

	stmt, err := conn.Prepare("DBCC TRACESTATUS (1224);")
	if err != nil {
		log.Fatal("Prepare failed:", err.Error())
	}
	defer stmt.Close()

	row := stmt.QueryRow()
	var traceFlag string
	var status byte
	var global byte
	var session byte
	err = row.Scan(&traceFlag, &status, &global, &session)
	if err != nil {
		log.Fatal("Scan failed:", err.Error())
	}
	//	log.Printf("server:%s\n", server)
	//	log.Printf("traceFlag:%s\n", traceFlag)
	//	log.Printf("status:%d\n", status)
	//	log.Printf("global:%d\n", global)
	//	log.Printf("session:%d\n", session)

	if status == 1 {
		return "ok"
	} else {
		return "fail"
	}
}
