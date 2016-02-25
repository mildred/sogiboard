package main

import (
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"sort"
	"strconv"
	"time"
)

func handleWebError(w http.ResponseWriter, err error, status int) {
	w.WriteHeader(status)
	_, err = w.Write([]byte(err.Error()))
	if err != nil {
		log.Println(err)
	}
}

type serv struct {
}

func dateadd(date string, days float32) (string, error) {
	d, err := time.Parse("02/01/2006", date)
	if err != nil {
		return "", err
	}
	d = d.Add(time.Duration(days*24) * time.Hour)
	return d.Format("02/01/2006"), nil
}

func (s *serv) ServeHTTP(w http.ResponseWriter, req *http.Request) {

	gg_sheet := req.URL.Query().Get("google-sheet")
	sheet_nr := req.URL.Query().Get("sheet")
	format := req.URL.Query().Get("format")
	url := "https://docs.google.com/spreadsheets/d/" + gg_sheet + "/pub?output=csv&gid=" + sheet_nr

	res, err := http.Get(url)
	if err != nil {
		handleWebError(w, err, http.StatusBadGateway)
		return
	}

	in := csv.NewReader(res.Body)
	data, err := in.ReadAll()
	if err != nil {
		handleWebError(w, err, http.StatusInternalServerError)
		return
	}

	var timesheet map[string][]string = map[string][]string{}
	var dates []string
	var people []string

	log.Println(url)
	for col := 3; col < len(data[0]); col++ {
		start := 0
		var duration float32
		dates = nil
		var resdata []string
		for _, line := range data {
			if start == 0 && line[0] == "Feuille de temps" {
				start = 1
			} else if start == 2 || start == 1 && line[0] != "" {
				start = 2
				d, err := dateadd(data[0][0], duration)
				if err != nil {
					handleWebError(w, err, http.StatusInternalServerError)
				}
				resdata = append(resdata, line[col])
				dates = append(dates, d)
				duration += 0.25
			}
		}
		timesheet[data[0][col]] = resdata
		people = append(people, data[0][col])
	}

	sort.Strings(people)

	if format == "csv" {

		w.Header().Set("Content-Type", "text/csv; encoding=\"utf-8\"")
		out := csv.NewWriter(w)
		err := out.Write(append([]string{"Date"}, people...))
		if err != nil {
			handleWebError(w, err, http.StatusInternalServerError)
			return
		}

		for i, date := range dates {
			var line []string = []string{date}
			for _, person := range people {
				line = append(line, timesheet[person][i])
			}
			err := out.Write(line)
			if err != nil {
				handleWebError(w, err, http.StatusInternalServerError)
				return
			}
		}

		out.Flush()

	} else if format == "csv2" {

		w.Header().Set("Content-Type", "text/csv; encoding=\"utf-8\"")
		out := csv.NewWriter(w)
		err := out.Write([]string{"Date", "Nom", "Durée", "Tâche"})
		if err != nil {
			handleWebError(w, err, http.StatusInternalServerError)
			return
		}

		for _, person := range people {
			sheet := timesheet[person]
			for i, date := range dates {
				err := out.Write([]string{date, person, "0,5", sheet[i]})
				if err != nil {
					handleWebError(w, err, http.StatusInternalServerError)
					return
				}
			}
		}

		out.Flush()

	} else {
		out, err := json.Marshal(map[string]interface{}{
			"timesheet": timesheet,
			"date":      data[0][0],
			"dates":     dates,
		})
		if err != nil {
			handleWebError(w, err, http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json; encoding=\"utf-8\"")
		w.Write(out)
	}
}

func main() {
	var default_port int64 = 8080
	if p := os.Getenv("PORT"); p != "" {
		var err error
		default_port, err = strconv.ParseInt(p, 10, 32)
		if err != nil {
			log.Printf("Environment variable PORT=%v is not a number", p)
			default_port = 8080
		}
	}
	arg_port := flag.Int("port", int(default_port), "HTTP Port")
	flag.Parse()
	log.Printf("Starting server on port %d\n", *arg_port)
	srv := http.Server{
		Addr:    fmt.Sprintf(":%d", *arg_port),
		Handler: &serv{},
	}
	err := srv.ListenAndServe()
	if err != nil {
		log.Fatal(err)
	}
}
