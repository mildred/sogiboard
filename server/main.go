package server

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"github.com/jbenet/go-base58"
	"github.com/mildred/sogiboard/crypt"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"time"
)

const (
	result = `
	<hr/>
	<p>Encrypted query string: <tt>%s</tt></p>
	`
)

var t_form *template.Template

func init() {
	form, err := Asset("data/form.html")
	if err != nil {
		panic(err)
	}

	t_form, err = template.New("data/form.html").Parse(string(form))
	if err != nil {
		panic(err)
	}
}

type v_form struct {
	SecretKey       string
	Encrypted       string
	Decrypted       url.Values
	DecryptedString string
	URL             string
	Fields          url.Values
}

func newFormView() (res v_form) {
	res.Fields = url.Values{
		"docid":         []string{""},
		"sheet":         []string{""},
		"format":        []string{""},
		"match_project": []string{"^"},
	}
	return
}

func extractURL(req *http.Request) string {
	if req.TLS == nil {
		return "http://" + req.Host + req.URL.Path
	} else {
		return "https://" + req.Host + req.URL.Path
	}
}

func handleWebError(w http.ResponseWriter, err error, status int) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(status)
	_, err = w.Write([]byte(err.Error()))
	if err != nil {
		log.Println(err)
	}
}

type serv struct {
	skey   []byte
	client *http.Client
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

	if req.URL.Path == "/" && len(req.URL.RawQuery) > 0 {
		u, err := crypt.DecryptUrl(s.skey, req.URL)
		if err != nil {
			handleWebError(w, err, http.StatusBadRequest)
			return
		}

		log.Printf("Decrypted: %s", u.RawQuery)

		convertBoard(s.client, w, u)
	} else if req.URL.Path == "/" && req.Method == "GET" {
		showForm(w, req)
	} else if req.URL.Path == "/" && req.Method == "POST" {
		showResults(w, req)
	} else {
		handleWebError(w, fmt.Errorf("Not Found: %s", req.URL.Path), http.StatusNotFound)
	}

}

func showForm(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)

	v := newFormView()
	v.URL = extractURL(req)
	v.Fields["match_project"] = []string{"^"}
	if err := t_form.Execute(w, v); err != nil {
		log.Print(err)
	}
}

func showResults(w http.ResponseWriter, req *http.Request) {
	err := req.ParseForm()
	if err != nil {
		handleWebError(w, err, http.StatusBadRequest)
		return
	}

	v := newFormView()
	v.Fields = req.PostForm
	v.URL = extractURL(req)
	v.SecretKey = req.PostForm.Get("skey")

	skey := base58.Decode(v.SecretKey)

	_, encode := req.PostForm["encode"]
	_, decode := req.PostForm["decode"]

	var qs url.Values = req.PostForm
	qs.Del("skey")
	qs.Del("encode")
	qs.Del("decode")
	qs.Del("encrypted")

	if encode {
		v.Encrypted, err = crypt.EncryptQueryToBase58(skey, qs)
		if err != nil {
			handleWebError(w, err, http.StatusInternalServerError)
			return
		}

		v.DecryptedString, err = crypt.DecryptBase58RawQuery(skey, v.Encrypted)
		if err != nil {
			handleWebError(w, err, http.StatusInternalServerError)
			return
		}

		v.Decrypted, _ = url.ParseQuery(v.DecryptedString)

	} else {
		_ = decode

		decrypted, err := crypt.Decrypt(skey, []byte(base58.Decode(req.PostForm.Get("encrypted"))))
		if err != nil {
			handleWebError(w, err, http.StatusInternalServerError)
			return
		}
		v.DecryptedString = string(decrypted)

		v.Decrypted, _ = url.ParseQuery(v.DecryptedString)

		encrypted, err := crypt.Encrypt(skey, []byte(v.DecryptedString))
		if err != nil {
			handleWebError(w, err, http.StatusInternalServerError)
			return
		}
		v.Encrypted = base58.Encode(encrypted)
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)

	if err := t_form.Execute(w, v); err != nil {
		log.Print(err)
	}
}

func convertBoard(client *http.Client, w http.ResponseWriter, u *url.URL) {
	var err error
	var match_project *regexp.Regexp

	gg_sheet := u.Query().Get("docid")
	sheet_nr := u.Query().Get("sheet")
	format := u.Query().Get("format")
	url := "https://docs.google.com/spreadsheets/d/" + gg_sheet + "/pub?output=csv&gid=" + sheet_nr

	if project := u.Query().Get("match_project"); project != "" {
		match_project, err = regexp.Compile(project)
		if err != nil {
			handleWebError(w, err, http.StatusBadGateway)
			return
		}
	}

	res, err := client.Get(url)
	if err != nil {
		handleWebError(w, err, http.StatusBadGateway)
		return
	}

	log.Printf("Query: %s", url)

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
		err := out.Write([]string{"Date", "Nom", "Durée", "Tâche"})
		if err != nil {
			handleWebError(w, err, http.StatusInternalServerError)
			return
		}

		for _, person := range people {
			sheet := timesheet[person]
			for i, date := range dates {
				project := sheet[i]
				if match_project != nil && !match_project.MatchString(project) {
					continue
				}
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

func getSecretKey(e string) []byte {
	sk := base58.Decode(e)
	if len(sk) != crypt.BlockSize {
		log.Print("Could not decode secret key")
		return nil
	}

	return sk
}

func Init(listen string, client *http.Client, secret_key string) error {
	log.Printf("Starting server on %s\n", listen)
	srv := http.Server{
		Addr: listen,
		Handler: &serv{
			getSecretKey(secret_key),
			client,
		},
	}
	return srv.ListenAndServe()
}
