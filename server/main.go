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
	"strings"
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
		"docid":      []string{""},
		"sheet":      []string{""},
		"format":     []string{""},
		"match_task": []string{"^"},
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

func (s *serv) ServeHTTP(w http.ResponseWriter, req *http.Request) {

	w.Header().Set("Access-Control-Allow-Origin", "*")

	if req.URL.Path == "/" && len(req.URL.RawQuery) > 0 {
		query := req.URL.Query()
		secure := query.Get("s")
		values, err := crypt.DecryptBase58Query(s.skey, secure)
		if err != nil {
			handleWebError(w, err, http.StatusBadRequest)
			return
		}

		log.Printf("Decrypted: %s", values.Encode())

		if extensions, ok := values["x"]; ok {
			for _, ext := range extensions {
				if vals, ok := query[ext]; ok {
					values[ext] = vals
				}
			}
		}

		convertBoard(s.client, w, values)
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
	v.Fields["match_task"] = []string{"^"}
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

	_, encoderaw := req.PostForm["encoderaw"]
	_, encode := req.PostForm["encode"]
	_, decode := req.PostForm["decode"]

	encrypted := req.PostForm.Get("encrypted")
	decrypted := req.PostForm.Get("decrypted")

	var qs url.Values = req.PostForm
	qs.Del("skey")
	qs.Del("encode")
	qs.Del("decode")
	qs.Del("encrypted")
	qs.Del("decrypted")

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

	} else if encoderaw {

		v.Encrypted, err = crypt.EncryptRawQueryToBase58(skey, decrypted)
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

		dec, err := crypt.Decrypt(skey, []byte(base58.Decode(encrypted)))
		if err != nil {
			handleWebError(w, err, http.StatusInternalServerError)
			return
		}
		v.DecryptedString = string(dec)

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

type Occupation struct {
	Person   string
	Date     time.Time
	Duration float32
	Task     string
	Project  string
	Error    string
}

func convertBoard(client *http.Client, w http.ResponseWriter, v url.Values) {
	var err error
	var match_task *regexp.Regexp

	// List of times per person
	var people []string
	var occupations []Occupation

	format := v.Get("format")
	gg_sheet := v.Get("docid")

	for _, sheet_nr := range v["sheet"] {

		if sheet_nr == "" {
			continue
		}

		url := "https://docs.google.com/spreadsheets/d/" + gg_sheet + "/pub?output=csv&gid=" + sheet_nr

		if project := v.Get("match_project"); project != "" {
			match_task, err = regexp.Compile(project)
			if err != nil {
				handleWebError(w, err, http.StatusBadGateway)
				return
			}
		} else if project := v.Get("match_task"); project != "" {
			match_task, err = regexp.Compile(project)
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
		log.Println(url)

		var projects map[string]string
		for line := 1; line < len(data); line++ {
			project := data[line][0]
			if project == "" && projects == nil {
				projects = make(map[string]string)
			} else if projects != nil && len(projects) > 0 && project == "" {
				break
			} else if projects != nil && project != "" {
				keyword := data[line][1]
				var pattern string
				if keyword == "" {
					pattern = "^" + regexp.QuoteMeta(project) + "$"
				} else {
					pattern = "^(" + regexp.QuoteMeta(project) + "|"
					for i := 0; i < len(keyword); i++ {
						c := keyword[i : i+1]
						if c == "*" {
							pattern += ".*"
						} else if c == "?" {
							pattern += "."
						} else if c == "~" && i+1 < len(keyword) && (keyword[i+1:i+2] == "?" || keyword[i+1:i+2] == "*") {
							pattern += regexp.QuoteMeta(keyword[i+1 : i+2])
							i++
						} else {
							pattern += regexp.QuoteMeta(c)
						}
					}
					pattern += ")$"
				}
				projects[project] = pattern
				//log.Printf("Project %#v: %#v", project, pattern)
			}
		}

		for col := 3; col < len(data[0]); col++ {
			start := 0
			var duration float32
			for _, line := range data {
				if start == 0 && line[0] == "Feuille de temps" {
					start = 1
				} else if start == 2 || start == 1 && line[0] != "" {
					start = 2

					d, err := time.Parse("02/01/2006", data[0][0])
					if err != nil {
						handleWebError(w, err, http.StatusInternalServerError)
					}
					d = d.Add(time.Duration(duration*24) * time.Hour)

					duration += 0.25

					if match_task != nil && !match_task.MatchString(line[col]) {
						continue
					}

					var task string = line[col]
					var prj_error, prj_name string

					for prj, pattern := range projects {
						matched, err := regexp.MatchString(pattern, task)
						if err != nil {
							panic(err)
						}
						if matched && prj_name == "" {
							prj_name = prj
						} else if matched {
							if prj_error == "" {
								prj_error = "Multiple project matches: " + prj_name
							}
							prj_error += ", " + prj
							prj_name = "error"
						}
					}

					occ := Occupation{
						Person:   data[0][col],
						Date:     d,
						Duration: 0.25,
						Task:     task,
						Project:  prj_name,
						Error:    prj_error,
					}

					if len(occupations) > 0 {
						last_occ := occupations[len(occupations)-1]
						if last_occ.Person == occ.Person &&
							last_occ.Task == occ.Task &&
							last_occ.Date.Format("02/01/2006") == occ.Date.Format("02/01/2006") {
							occ.Duration += last_occ.Duration
							occupations = occupations[:len(occupations)-1]
						}
					}

					occupations = append(occupations, occ)
				}
			}
			people = append(people, data[0][col])
		}

	}

	sort.Strings(people)

	if format == "csv" {

		w.Header().Set("Content-Type", "text/csv; encoding=\"utf-8\"")
		out := csv.NewWriter(w)
		err := out.Write([]string{"Date", "Nom", "Durée", "Tâche", "Projet"})
		if err != nil {
			handleWebError(w, err, http.StatusInternalServerError)
			return
		}

		for _, occ := range occupations {
			err := out.Write([]string{
				occ.Date.Format("02/01/2006"),
				occ.Person,
				strings.Replace(fmt.Sprintf("%g", occ.Duration), ".", ",", -1),
				occ.Task,
				occ.Project})
			if err != nil {
				handleWebError(w, err, http.StatusInternalServerError)
				return
			}
		}

		out.Flush()

	} else {
		out, err := json.Marshal(map[string]interface{}{
			"people":      people,
			"occupations": occupations,
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
