package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"

	"github.com/codegangsta/negroni"
	"github.com/gorilla/mux"
)

func init() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
}

func getwd() string {
	wd, err := os.Getwd()
	if err != nil {
		log.Fatalln("cannot getwd:", err)
	}
	return wd
}

func isExist(fp string) bool {
	_, err := os.Stat(fp)
	return err == nil || os.IsExist(err)
}

func getArgs() (int64, string) {
	args := os.Args
	l := len(args)

	portstr := "8888"
	authstr := ""
	if l >= 2 {
		// http port given
		portstr = args[1]
	}

	if l >= 3 {
		authstr = args[2]
	}

	port, err := strconv.ParseInt(portstr, 10, 64)
	if err != nil {
		log.Fatalln("cannot parse port:", portstr)
	}

	return port, authstr
}

func main() {
	port, auth := getArgs()
	startHttp(port, auth)
}

func startHttp(port int64, authstr string) {
	r := mux.NewRouter().StrictSlash(false)

	r.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "pong")
	})

	r.HandleFunc("/print", func(w http.ResponseWriter, r *http.Request) {
		fmt.Printf("r.RequestURI: %s\n", r.RequestURI)
		fmt.Printf("r.URL.Path: %s\n", r.URL.Path)
		fmt.Printf("r.URL.Host: %s\n", r.URL.Host)
		fmt.Printf("r.URL.Hostname(): %s\n", r.URL.Hostname())
		fmt.Printf("r.Method: %s\n", r.Method)
		fmt.Printf("r.URL.Scheme: %s\n", r.URL.Scheme)
		fmt.Printf("SimpleHTTPD pid: %d\n", os.Getpid())

		fmt.Printf("\nHeaders: \n")
		for name, values := range r.Header {
			if len(values) == 1 {
				fmt.Printf("%s: %v\n", name, values[0])
				continue
			}

			fmt.Println(name)
			for i := 0; i < len(values); i++ {
				fmt.Printf("  - #%d: %s\n", i, values[i])
			}
		}

		fmt.Printf("\nPayload: \n")
		defer r.Body.Close()
		bs, _ := io.ReadAll(r.Body)
		fmt.Println(string(bs))
		fmt.Fprintln(w, "ok")
	})

	r.HandleFunc("/request", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "r.RequestURI: %s\n", r.RequestURI)
		fmt.Fprintf(w, "r.URL.Path: %s\n", r.URL.Path)
		fmt.Fprintf(w, "r.URL.Host: %s\n", r.URL.Host)
		fmt.Fprintf(w, "r.URL.Hostname(): %s\n", r.URL.Hostname())
		fmt.Fprintf(w, "r.Method: %s\n", r.Method)
		fmt.Fprintf(w, "r.URL.Scheme: %s\n", r.URL.Scheme)
		fmt.Fprintf(w, "SimpleHTTPD pid: %d\n", os.Getpid())

		fmt.Fprintf(w, "\nHeaders: \n")
		for name, values := range r.Header {
			if len(values) == 1 {
				fmt.Fprintf(w, "%s: %v\n", name, values[0])
				continue
			}

			fmt.Fprintln(w, name)
			for i := 0; i < len(values); i++ {
				fmt.Fprintf(w, "  - #%d: %s\n", i, values[i])
			}
		}

		fmt.Fprintf(w, "\nPayload: \n")
		defer r.Body.Close()
		bs, _ := io.ReadAll(r.Body)
		fmt.Fprintln(w, string(bs))
	})

	r.HandleFunc("/run", func(w http.ResponseWriter, r *http.Request) {
		if authstr == "" {
			http.Error(w, "not supported. use ./httpd <port> <auth-string> to enable", 200)
			return
		}

		if r.Body == nil {
			http.Error(w, "body is nil", http.StatusBadRequest)
			return
		}

		auth := r.Header.Get("Authorization")
		if strings.TrimSpace(auth) == "" {
			http.Error(w, "Authorization is blank", http.StatusBadRequest)
			return
		}

		auth = strings.TrimPrefix(auth, "Bearer ")
		if auth != authstr {
			http.Error(w, "Authorization invalid", http.StatusForbidden)
			return
		}

		defer r.Body.Close()
		bs, _ := io.ReadAll(r.Body)
		sh := string(bs)

		cmd := exec.Command("sh", "-c", sh)
		output, err := cmd.CombinedOutput()
		if err != nil {
			http.Error(w, fmt.Sprintf("exec `%s` fail: %v", sh, err), 200)
			return
		}

		w.Write(output)
	})

	r.HandleFunc("/upload", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			w.Write([]byte(UPLOAD_HTML))
			return
		}

		r.ParseMultipartForm(32 << 20)
		files := r.MultipartForm.File["files"]
		len := len(files)
		for i := 0; i < len; i++ {
			file, err := files[i].Open()
			if err != nil {
				http.Error(w, err.Error(), 200)
				return
			}

			defer file.Close()

			filepath := "./" + files[i].Filename
			if isExist(filepath) {
				err = os.Remove(filepath)
				if err != nil {
					http.Error(w, err.Error(), 200)
					return
				}
			}

			cur, err := os.Create(filepath)
			if err != nil {
				http.Error(w, err.Error(), 200)
				return
			}

			defer cur.Close()

			io.Copy(cur, file)
		}

		http.Error(w, "SUCCESS", 200)
	})

	r.PathPrefix("/").Handler(http.FileServer(http.Dir("./")))

	n := negroni.New()
	n.UseHandler(r)

	log.Println("listening http on", port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", port), n))
}

var UPLOAD_HTML = `<!doctype html>
<html lang="en">

<head>
    <meta charset="UTF-8">
    <title>SimpleHTTPD</title>
    <style type="text/css">
    * {
        font-family: "Microsoft YaHei";
        font-size: 12px;
    }

    .btn {
        display: inline-block;
        padding: 6px 12px;
        margin-bottom: 0;
        font-size: 14px;
        font-weight: 400;
        line-height: 1.42857143;
        text-align: center;
        white-space: nowrap;
        vertical-align: middle;
        -ms-touch-action: manipulation;
        touch-action: manipulation;
        cursor: pointer;
        -webkit-user-select: none;
        -moz-user-select: none;
        -ms-user-select: none;
        user-select: none;
        background-image: none;
        border: 1px solid transparent;
        border-radius: 4px;
    }

    .btn-primary {
        color: #fff;
        background-color: #337ab7;
        border-color: #2e6da4;
    }

    .btn-xs {
        padding: 1px 8px;
        font-size: 12px;
        line-height: 1.5;
        border-radius: 3px;
    }
    </style>
</head>

<body>
    <form action="/upload" method="post" enctype="multipart/form-data">
        Upload files:
        <input type="file" name="files" multiple="multiple" />
        <input type="submit" name="submit" value="Upload" class="btn btn-primary btn-xs">
    </form>
</body>

</html>
`
