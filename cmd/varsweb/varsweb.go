//////////////////////////////////////////////////////////////////////////////////////
//                                                                                  //
//    VARS (Vulnerability Analysis Reference System) is software used to track      //
//    vulnerabilities from discovery through analysis to mitigation.                //
//    Copyright (C) 2017  Christian Belk                                            //
//                                                                                  //
//    This program is free software: you can redistribute it and/or modify          //
//    it under the terms of the GNU General Public License as published by          //
//    the Free Software Foundation, either version 3 of the License, or             //
//    (at your option) any later version.                                           //
//                                                                                  //
//    This program is distributed in the hope that it will be useful,               //
//    but WITHOUT ANY WARRANTY; without even the implied warranty of                //
//    MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the                 //
//    GNU General Public License for more details.                                  //
//                                                                                  //
//    See the full License here: https://github.com/cbelk/vars/blob/master/LICENSE  //
//                                                                                  //
//////////////////////////////////////////////////////////////////////////////////////

package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"sort"
	"strconv"
	"strings"

	"github.com/alexedwards/scs"
	"github.com/cbelk/vars"
	"github.com/cbelk/vars/pkg/varsapi"
	"github.com/julienschmidt/httprouter"
)

const (
	AdminUser      = 0
	PrivilegedUser = 1
	StandardUser   = 2
	Reporter       = 3
)

var (
	Conf           vars.Config
	db             *sql.DB
	sessionManager *scs.Manager
)

// User will hold whether the user is authed and their vars.Employee object.
type User struct {
	Authed bool
	Emp    *vars.Employee
}

func main() {
	// Setup logging
	SetupLogging()

	// Compile the regexp used to obfuscate session cookie in logging
	setupRegex()

	// Read in the configurations
	ReadVarsConfig()
	ReadWebConfig()

	// Load the authentication plugin
	LoadAuth()

	// Load the reports
	LoadReports()

	// Load templates
	LoadTemplates()

	// Start the database connection
	var err error
	db, err = vars.ConnectDB(&Conf)
	if err != nil {
		logError.Fatal(err)
	}
	defer vars.CloseDB(db)

	// Create Session Manager
	sessionManager = scs.NewCookieManager(webConf.Skey)
	sessionManager.Secure(true)

	// Set paths
	router := httprouter.New()
	router.GET("/", handleIndex)
	router.GET("/login", handleLoginGet)
	router.POST("/login", handleLoginPost)
	router.GET("/logout", handleLogout)
	router.PUT("/employee", handleEmployeeAdd)
	router.GET("/employee", handleEmployeePage)
	router.DELETE("/employee/:emp", handleEmployeeDelete)
	router.GET("/employee/:emp", handleEmployees)
	router.GET("/employee/:emp/:id", handleEmployees)
	router.POST("/employee/:emp/:field", handleEmployeePost)
	router.GET("/notes/:vuln", handleNotes)
	router.POST("/notes/:noteid", handleNotesPost)
	router.GET("/report", handleReportPage)
	router.GET("/report/:report", handleReport)
	router.PUT("/system", handleSystemAdd)
	router.GET("/system", handleSystemPage)
	router.GET("/system/:sys", handleSystems)
	router.DELETE("/system/:sys", handleSystemDelete)
	router.POST("/system/:sys/:field", handleSystemPost)
	router.PUT("/vulnerability", handleVulnerabilityAdd)
	router.GET("/vulnerability", handleVulnerabilityPage)
	router.GET("/vulnerability/:vuln", handleVulnerabilities)
	router.GET("/vulnerability/:vuln/:field", handleVulnerabilityField)
	router.PUT("/vulnerability/:vuln/:field", handleVulnerabilityPut)
	router.POST("/vulnerability/:vuln/:field", handleVulnerabilityPost)
	router.DELETE("/vulnerability/:vuln/:field", handleVulnerabilityDelete)
	router.POST("/vulnerability/:vuln/:field/:item", handleVulnerabilityPost)
	router.DELETE("/vulnerability/:vuln/:field/:item", handleVulnerabilityDelete)

	// Serve css, javascript and images
	router.ServeFiles("/styles/*filepath", http.Dir(fmt.Sprintf("%s/styles", webConf.WebRoot)))
	router.ServeFiles("/scripts/*filepath", http.Dir(fmt.Sprintf("%s/scripts", webConf.WebRoot)))
	router.ServeFiles("/images/*filepath", http.Dir(fmt.Sprintf("%s/images", webConf.WebRoot)))

	logError.Fatal(http.ListenAndServe(fmt.Sprintf(":%s", webConf.Port), router))
}

// handleIndex serves the main page.
func handleIndex(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	logRequest(r)
	user, err := getSession(r)
	if err != nil {
		logError.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if user.Authed {
		s := struct {
			Page string
			User interface{}
		}{"index", user}
		w.Header().Add("Content-Type", "text/html")
		err := templates.Lookup("index").Execute(w, s)
		if err != nil {
			logError.Printf("Error with templating while performing lookup on %s\n", "index")
			http.Error(w, "Error with templating", http.StatusInternalServerError)
			return
		}
	} else {
		http.Redirect(w, r, "/login", http.StatusFound)
	}
}

// handleLoginGet serves the login page.
func handleLoginGet(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	user, err := getSession(r)
	if err != nil {
		logError.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if user.Authed {
		http.Redirect(w, r, "/", http.StatusFound)
	} else {
		w.Header().Add("Content-Type", "text/html")
		err := templates.Lookup("login").Execute(w, "login")
		if err != nil {
			logError.Printf("Error with templating while performing lookup on %s\n", "login")
			http.Error(w, "Error with templating", http.StatusInternalServerError)
			return
		}
	}
}

// handleLoginPost uses the Authenticate function of the auth plugin to validate the user credentials.
func handleLoginPost(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	var user User
	u := r.FormValue("username")
	p := r.FormValue("password")
	authed, err := authenticate(u, p)
	if err != nil {
		logError.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	user.Authed = authed
	session := sessionManager.Load(r)
	if user.Authed {
		if u == "VARSremoved" {
			logInfo.Printf("Unauthorized attempt to log in. Username: %s | Client IP/Port: %s\n", u, r.RemoteAddr)
			w.Header().Add("Content-Type", "text/html")
			err := templates.Lookup("notauthorized-removed").Execute(w, nil)
			if err != nil {
				logError.Printf("Error with templating while performing lookup on %s\n", "notauthorized-removed")
				http.Error(w, "Error with templating", http.StatusInternalServerError)
				return
			}
			return
		}
		emp, err := varsapi.GetEmployeeByUsername(u)
		if err != nil {
			if varsapi.IsNoRowsError(err) {
				logInfo.Printf("Unauthorized attempt to log in. Username: %s | Client IP/Port: %s\n", u, r.RemoteAddr)
				w.Header().Add("Content-Type", "text/html")
				err := templates.Lookup("notauthorized-removed").Execute(w, "")
				if err != nil {
					logError.Printf("Error with templating while performing lookup on %s\n", "notauthorized-removed")
					http.Error(w, "Error with templating", http.StatusInternalServerError)
					return
				}
				return
			}
			logError.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		user.Emp = emp
		err = session.PutObject(w, "user", user)
		if err != nil {
			logError.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, "/", http.StatusFound)
	} else {
		logInfo.Printf("Failed attempt to log in. Username: %s | Client IP/Port: %s\n", u, r.RemoteAddr)
		err = session.PutObject(w, "user", user)
		if err != nil {
			logError.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		err := templates.Lookup("login-failed").Execute(w, "login")
		if err != nil {
			logError.Printf("Error with templating while performing lookup on %s\n", "login-failed")
			http.Error(w, "Error with templating", http.StatusInternalServerError)
			return
		}
	}
}

// handleLogout destroys the session and redirects to the login page.
func handleLogout(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	session := sessionManager.Load(r)
	err := session.Destroy(w)
	if err != nil {
		logError.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/login", http.StatusFound)
}

// handleEmployeePage serves the employee page outline
func handleEmployeePage(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	logRequest(r)
	user, err := getSession(r)
	if err != nil {
		logError.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if user.Authed {
		if user.Emp.Level == AdminUser {
			s := struct {
				Page string
				User interface{}
			}{"emp", user}
			w.Header().Add("Content-Type", "text/html")
			err := templates.Lookup("emps").Execute(w, s)
			if err != nil {
				logError.Printf("Error with templating while performing lookup on %s\n", "emps")
				http.Error(w, "Error with templating", http.StatusInternalServerError)
				return
			}
		} else {
			err := templates.Lookup("notauthorized-get").Execute(w, user)
			if err != nil {
				logError.Printf("Error with templating while performing lookup on %s\n", "notauthorized-get")
				http.Error(w, "Error with templating", http.StatusInternalServerError)
				return
			}
		}
	} else {
		http.Redirect(w, r, "/login", http.StatusFound)
	}
}

// handleEmployees serves the employee objects
func handleEmployees(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	logRequest(r)
	user, err := getSession(r)
	if err != nil {
		logError.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	e := ps.ByName("emp")
	if user.Authed {
		if user.Emp.Level == AdminUser {
			switch e {
			case "all":
				emps, err := varsapi.GetEmployees()
				if err != nil {
					logError.Println(err)
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				var data []interface{}
				for _, emp := range emps {
					s := struct {
						ID        int64
						FirstName string
						LastName  string
						Email     string
						UserName  string
						Level     int
					}{emp.ID, emp.FirstName, emp.LastName, emp.Email, emp.UserName, emp.Level}
					data = append(data, s)
				}
				err = json.NewEncoder(w).Encode(data)
				if err != nil {
					logError.Println(err)
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
			case "active":
				emps, err := varsapi.GetEmployees()
				if err != nil {
					logError.Println(err)
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				var data []interface{}
				for _, emp := range emps {
					if emp.UserName != "VARSremoved" {
						s := struct {
							ID        int64
							FirstName string
							LastName  string
							Email     string
							UserName  string
							Level     int
						}{emp.ID, emp.FirstName, emp.LastName, emp.Email, emp.UserName, emp.Level}
						data = append(data, s)
					}
				}
				err = json.NewEncoder(w).Encode(data)
				if err != nil {
					logError.Println(err)
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
			case "removed":
				emps, err := varsapi.GetEmployees()
				if err != nil {
					logError.Println(err)
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				var data []interface{}
				for _, emp := range emps {
					if emp.UserName == "VARSremoved" {
						s := struct {
							ID        int64
							FirstName string
							LastName  string
							Email     string
							UserName  string
							Level     int
						}{emp.ID, emp.FirstName, emp.LastName, emp.Email, emp.UserName, emp.Level}
						data = append(data, s)
					}
				}
				err = json.NewEncoder(w).Encode(data)
				if err != nil {
					logError.Println(err)
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
			case "list":
				emps, err := varsapi.GetEmployees()
				if err != nil {
					logError.Println(err)
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				var data []interface{}
				for _, emp := range emps {
					name := fmt.Sprintf("%s %s", emp.FirstName, emp.LastName)
					s := struct {
						ID   int64
						Name string
					}{emp.ID, name}
					data = append(data, s)
				}
				err = json.NewEncoder(w).Encode(data)
				if err != nil {
					logError.Println(err)
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
			case "name":
				i := ps.ByName("id")
				eid, err := strconv.Atoi(i)
				if err != nil {
					w.WriteHeader(http.StatusNotFound)
					return
				}
				emp, err := varsapi.GetEmployeeByID(int64(eid))
				if err != nil {
					logError.Println(err)
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				name := fmt.Sprintf("%s %s", emp.FirstName, emp.LastName)
				s := struct {
					Name string
				}{name}
				err = json.NewEncoder(w).Encode(s)
				if err != nil {
					logError.Println(err)
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
			default:
				eid, err := strconv.Atoi(e)
				if err != nil {
					w.WriteHeader(http.StatusNotFound)
					return
				}
				emp, err := varsapi.GetEmployeeByID(int64(eid))
				if err != nil {
					logError.Println(err)
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				err = json.NewEncoder(w).Encode(emp)
				if err != nil {
					logError.Println(err)
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
			}
		} else if user.Emp.Level == PrivilegedUser || user.Emp.Level == StandardUser {
			if e == "name" {
				i := ps.ByName("id")
				eid, err := strconv.Atoi(i)
				if err != nil {
					w.WriteHeader(http.StatusNotFound)
					return
				}
				emp, err := varsapi.GetEmployeeByID(int64(eid))
				if err != nil {
					logError.Println(err)
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				name := fmt.Sprintf("%s %s", emp.FirstName, emp.LastName)
				s := struct {
					Name string
				}{name}
				err = json.NewEncoder(w).Encode(s)
				if err != nil {
					logError.Println(err)
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
			} else if e == "list" {
				if user.Emp.Level == PrivilegedUser {
					emps, err := varsapi.GetEmployees()
					if err != nil {
						logError.Println(err)
						w.WriteHeader(http.StatusInternalServerError)
						return
					}
					var data []interface{}
					for _, emp := range emps {
						name := fmt.Sprintf("%s %s", emp.FirstName, emp.LastName)
						s := struct {
							ID   int64
							Name string
						}{emp.ID, name}
						data = append(data, s)
					}
					err = json.NewEncoder(w).Encode(data)
					if err != nil {
						logError.Println(err)
						w.WriteHeader(http.StatusInternalServerError)
						return
					}
				} else {
					w.WriteHeader(http.StatusUnauthorized)
					return
				}
			} else {
				w.WriteHeader(http.StatusTeapot)
				return
			}
		} else {
			err := templates.Lookup("notauthorized-get").Execute(w, user)
			if err != nil {
				logError.Printf("Error with templating while performing lookup on %s\n", "notauthorized-get")
				http.Error(w, "Error with templating", http.StatusInternalServerError)
				return
			}
		}
	} else {
		http.Redirect(w, r, "/login", http.StatusFound)
	}
}

// handleEmployeeAdd adds the new employee to VARS
func handleEmployeeAdd(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	logRequest(r)
	user, err := getSession(r)
	if err != nil {
		logError.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if user.Authed {
		if user.Emp.Level == AdminUser {
			fname := r.FormValue("firstname")
			lname := r.FormValue("lastname")
			email := r.FormValue("email")
			uname := r.FormValue("username")
			l := r.FormValue("level")
			level, err := strconv.Atoi(l)
			if err != nil {
				logError.Println(err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			emp := varsapi.CreateEmployee(fname, lname, email, uname, level)
			err = varsapi.AddEmployee(db, emp)
			if err != nil {
				logError.Println(err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			ist := struct {
				ID int64
			}{emp.ID}
			err = json.NewEncoder(w).Encode(ist)
			if err != nil {
				logError.Println(err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
		} else {
			err := templates.Lookup("notauthorized-get").Execute(w, user)
			if err != nil {
				logError.Printf("Error with templating while performing lookup on %s\n", "notauthorized-get")
				http.Error(w, "Error with templating", http.StatusInternalServerError)
				return
			}
		}
	} else {
		http.Redirect(w, r, "/login", http.StatusFound)
	}
}

func handleEmployeeDelete(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	logRequest(r)
	user, err := getSession(r)
	if err != nil {
		logError.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	e := ps.ByName("emp")
	eid, err := strconv.Atoi(e)
	if err != nil {
		logError.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if user.Authed {
		if user.Emp.Level == AdminUser {
			err = varsapi.DeleteEmployee(db, int64(eid))
			if err != nil {
				logError.Println(err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
		}
	} else {
		http.Redirect(w, r, "/login", http.StatusFound)
	}
}

func handleEmployeePost(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	logRequest(r)
	user, err := getSession(r)
	if err != nil {
		logError.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	e := ps.ByName("emp")
	eid, err := strconv.Atoi(e)
	if err != nil {
		logError.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	field := ps.ByName("field")
	if user.Authed {
		switch field {
		case "name":
			if user.Emp.Level == AdminUser {
				fname := r.FormValue("firstname")
				lname := r.FormValue("lastname")
				err := varsapi.UpdateEmployeeName(db, int64(eid), fname, lname)
				if err != nil {
					logError.Println(err)
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				w.WriteHeader(http.StatusOK)
				return
			} else {
				w.WriteHeader(http.StatusUnauthorized)
			}
		case "email":
			if user.Emp.Level == AdminUser {
				email := r.FormValue("email")
				err := varsapi.UpdateEmployeeEmail(db, int64(eid), email)
				if err != nil {
					logError.Println(err)
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				w.WriteHeader(http.StatusOK)
				return
			} else {
				w.WriteHeader(http.StatusUnauthorized)
			}
		case "username":
			if user.Emp.Level == AdminUser {
				username := r.FormValue("username")
				err := varsapi.UpdateEmployeeUsername(db, int64(eid), username)
				if err != nil {
					logError.Println(err)
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				w.WriteHeader(http.StatusOK)
				return
			} else {
				w.WriteHeader(http.StatusUnauthorized)
			}
		case "level":
			if user.Emp.Level == AdminUser {
				l := r.FormValue("level")
				level, err := strconv.Atoi(l)
				if err != nil {
					logError.Println(err)
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				err = varsapi.UpdateEmployeeLevel(db, int64(eid), level)
				if err != nil {
					logError.Println(err)
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				w.WriteHeader(http.StatusOK)
				return
			} else {
				w.WriteHeader(http.StatusUnauthorized)
			}
		default:
			w.WriteHeader(http.StatusTeapot)
			return
		}
	}
}

func handleNotes(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	logRequest(r)
	user, err := getSession(r)
	if err != nil {
		logError.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if user.Authed {
		if user.Emp.Level <= StandardUser {
			v := ps.ByName("vuln")
			vid, err := strconv.Atoi(v)
			if err != nil {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			ns, err := varsapi.GetNotes(int64(vid))
			if err != nil {
				if varsapi.IsNoRowsError(err) {
					w.WriteHeader(http.StatusOK)
					return
				}
				logError.Println(err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			var notes []interface{}
			for _, n := range ns {
				canEdit := user.Emp.ID == n.EmpID
				employee, err := varsapi.GetEmployeeByID(n.EmpID)
				if err != nil {
					logError.Println(err)
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				note := struct {
					Nid      int64
					Emp      string
					Added    string
					Note     string
					Editable bool
				}{n.ID, fmt.Sprintf("%v %v", employee.FirstName, employee.LastName), n.Added.Format("Mon, 02 Jan 2006 15:04:05"), n.Note, canEdit}
				notes = append(notes, note)
			}
			err = json.NewEncoder(w).Encode(notes)
			if err != nil {
				logError.Println(err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
		} else {
			err := templates.Lookup("notauthorized-get").Execute(w, user)
			if err != nil {
				logError.Printf("Error with templating while performing lookup on %s\n", "notauthorized-get")
				http.Error(w, "Error with templating", http.StatusInternalServerError)
				return
			}
		}
	} else {
		http.Redirect(w, r, "/login", http.StatusFound)
	}
}

func handleNotesPost(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	logRequest(r)
	user, err := getSession(r)
	if err != nil {
		logError.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if user.Authed {
		n := ps.ByName("noteid")
		nid, err := strconv.Atoi(n)
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		author, err := varsapi.GetNoteAuthor(int64(nid))
		if err != nil {
			logError.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if user.Emp.Level <= StandardUser && user.Emp.ID == author {
			note := r.FormValue("note")
			err = varsapi.UpdateNote(db, int64(nid), note)
			if err != nil {
				logError.Println(err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			} else {
				w.WriteHeader(http.StatusOK)
			}
		} else {
			err := templates.Lookup("notauthorized-get").Execute(w, user)
			if err != nil {
				logError.Printf("Error with templating while performing lookup on %s\n", "notauthorized-get")
				http.Error(w, "Error with templating", http.StatusInternalServerError)
				return
			}
		}
	} else {
		http.Redirect(w, r, "/login", http.StatusFound)
	}
}

// handleReportPage serves the system page outline
func handleReportPage(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	logRequest(r)
	user, err := getSession(r)
	if err != nil {
		logError.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if user.Authed {
		s := struct {
			Page string
			User interface{}
		}{"report", user}
		w.Header().Add("Content-Type", "text/html")
		err := templates.Lookup("report").Execute(w, s)
		if err != nil {
			logError.Printf("Error with templating while performing lookup on %s\n", "report")
			http.Error(w, "Error with templating", http.StatusInternalServerError)
			return
		}
	} else {
		http.Redirect(w, r, "/login", http.StatusFound)
	}
}

func handleReport(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	logRequest(r)
	user, err := getSession(r)
	if err != nil {
		logError.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if user.Authed {
		name := ps.ByName("report")
		switch name {
		case "list":
			var data []string
			for name, _ := range reports {
				data = append(data, name)
			}
			err = json.NewEncoder(w).Encode(data)
			if err != nil {
				logError.Println(err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
		default:
			_, ok := reports[name]
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			gen, err := reports[name].Lookup("GenerateReport")
			if err != nil {
				logError.Println(err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			g, ok := gen.(func() (string, error))
			if !ok {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			rep, err := g()
			if err != nil {
				logError.Println(err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			w.Header().Add("Content-Type", "text/html")
			w.Write([]byte(rep))
		}
	} else {
		http.Redirect(w, r, "/login", http.StatusFound)
	}
}

// handleSystemAdd adds the new system to VARS
func handleSystemAdd(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	logRequest(r)
	user, err := getSession(r)
	if err != nil {
		logError.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if user.Authed {
		if user.Emp.Level <= StandardUser {
			name := r.FormValue("name")
			tp := r.FormValue("type")
			opsys := r.FormValue("os")
			loc := r.FormValue("location")
			desc := r.FormValue("description")
			sys := varsapi.CreateSystem(name, tp, opsys, loc, desc, "active")
			err = varsapi.AddSystem(db, sys)
			if err != nil {
				if varsapi.IsNameNotAvailableError(err) {
					w.WriteHeader(http.StatusNotAcceptable)
					return
				}
				logError.Println(err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			ist := struct {
				ID int64
			}{sys.ID}
			err = json.NewEncoder(w).Encode(ist)
			if err != nil {
				logError.Println(err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
		} else {
			err := templates.Lookup("notauthorized-get").Execute(w, user)
			if err != nil {
				logError.Printf("Error with templating while performing lookup on %s\n", "notauthorized-get")
				http.Error(w, "Error with templating", http.StatusInternalServerError)
				return
			}
		}
	} else {
		http.Redirect(w, r, "/login", http.StatusFound)
	}
}

// handleSystemPage serves the system page outline
func handleSystemPage(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	logRequest(r)
	user, err := getSession(r)
	if err != nil {
		logError.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if user.Authed {
		if user.Emp.Level <= StandardUser {
			s := struct {
				Page string
				User interface{}
			}{"sys", user}
			w.Header().Add("Content-Type", "text/html")
			err := templates.Lookup("sys").Execute(w, s)
			if err != nil {
				logError.Printf("Error with templating while performing lookup on %s\n", "sys")
				http.Error(w, "Error with templating", http.StatusInternalServerError)
				return
			}
		} else {
			err := templates.Lookup("notauthorized-get").Execute(w, user)
			if err != nil {
				logError.Printf("Error with templating while performing lookup on %s\n", "notauthorized-get")
				http.Error(w, "Error with templating", http.StatusInternalServerError)
				return
			}
		}
	} else {
		http.Redirect(w, r, "/login", http.StatusFound)
	}
}

func handleSystemDelete(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	logRequest(r)
	user, err := getSession(r)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	s := ps.ByName("sys")
	sid, err := strconv.Atoi(s)
	if err != nil {
		logError.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if user.Authed {
		if user.Emp.Level <= PrivilegedUser {
			err = varsapi.DeleteSystem(db, int64(sid))
			if err != nil {
				logError.Println(err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusUnauthorized)
		return
	} else {
		http.Redirect(w, r, "/login", http.StatusFound)
	}
}

func handleSystems(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	logRequest(r)
	user, err := getSession(r)
	if err != nil {
		logError.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	s := ps.ByName("sys")
	if user.Authed {
		if user.Emp.Level <= StandardUser {
			switch s {
			case "all":
				syss, err := varsapi.GetSystems()
				if err != nil {
					logError.Println(err)
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				err = json.NewEncoder(w).Encode(syss)
				if err != nil {
					logError.Println(err)
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
			case "active", "inactive":
				syss, err := varsapi.GetSystemsByState(s)
				if err != nil {
					logError.Println(err)
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				err = json.NewEncoder(w).Encode(syss)
				if err != nil {
					logError.Println(err)
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
			default:
				sid, err := strconv.Atoi(s)
				if err != nil {
					w.WriteHeader(http.StatusNotFound)
					return
				}
				sys, err := varsapi.GetSystem(int64(sid))
				if err != nil {
					logError.Println(err)
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				err = json.NewEncoder(w).Encode(sys)
				if err != nil {
					logError.Println(err)
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
			}
		} else {
			err := templates.Lookup("notauthorized-get").Execute(w, user)
			if err != nil {
				logError.Printf("Error with templating while performing lookup on %s\n", "notauthorized-get")
				http.Error(w, "Error with templating", http.StatusInternalServerError)
				return
			}
		}
	} else {
		http.Redirect(w, r, "/login", http.StatusFound)
	}
}

func handleSystemPost(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	logRequest(r)
	user, err := getSession(r)
	if err != nil {
		logError.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	s := ps.ByName("sys")
	sid, err := strconv.Atoi(s)
	if err != nil {
		logError.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	field := ps.ByName("field")
	if user.Authed {
		switch field {
		case "name":
			if user.Emp.Level <= StandardUser {
				name := r.FormValue("name")
				err := varsapi.UpdateSystemName(db, int64(sid), name)
				if err != nil {
					if varsapi.IsNameNotAvailableError(err) {
						w.WriteHeader(http.StatusNotAcceptable)
						return
					}
					logError.Println(err)
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				w.WriteHeader(http.StatusOK)
				return
			} else {
				w.WriteHeader(http.StatusUnauthorized)
			}
		case "type":
			if user.Emp.Level <= StandardUser {
				tp := r.FormValue("type")
				err := varsapi.UpdateSystemType(db, int64(sid), tp)
				if err != nil {
					logError.Println(err)
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				w.WriteHeader(http.StatusOK)
				return
			} else {
				w.WriteHeader(http.StatusUnauthorized)
			}
		case "os":
			if user.Emp.Level <= StandardUser {
				opsys := r.FormValue("os")
				err := varsapi.UpdateSystemOS(db, int64(sid), opsys)
				if err != nil {
					logError.Println(err)
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				w.WriteHeader(http.StatusOK)
				return
			} else {
				w.WriteHeader(http.StatusUnauthorized)
			}
		case "location":
			if user.Emp.Level <= StandardUser {
				loc := r.FormValue("location")
				err := varsapi.UpdateSystemLocation(db, int64(sid), loc)
				if err != nil {
					logError.Println(err)
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				w.WriteHeader(http.StatusOK)
				return
			} else {
				w.WriteHeader(http.StatusUnauthorized)
			}
		case "description":
			if user.Emp.Level <= StandardUser {
				desc := r.FormValue("description")
				err := varsapi.UpdateSystemDescription(db, int64(sid), desc)
				if err != nil {
					logError.Println(err)
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				w.WriteHeader(http.StatusOK)
				return
			} else {
				w.WriteHeader(http.StatusUnauthorized)
			}
		case "state":
			if user.Emp.Level <= StandardUser {
				state := r.FormValue("state")
				if state != "active" && state != "inactive" {
					w.WriteHeader(http.StatusNotAcceptable)
					return
				}
				err := varsapi.UpdateSystemState(db, int64(sid), state)
				if err != nil {
					logError.Println(err)
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				w.WriteHeader(http.StatusOK)
				return
			} else {
				w.WriteHeader(http.StatusUnauthorized)
			}
		default:
			w.WriteHeader(http.StatusTeapot)
			return
		}
	}
}

// handleVulnerabilityAdd adds the new vuln to VARS
func handleVulnerabilityAdd(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	logRequest(r)
	user, err := getSession(r)
	if err != nil {
		logError.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if user.Authed {
		if user.Emp.Level <= PrivilegedUser {
			name := r.FormValue("name")
			summ := r.FormValue("summary")
			cvss := r.FormValue("cvssScore")
			cvsl := r.FormValue("cvssLink")
			corp := r.FormValue("corpscore")
			test := r.FormValue("test")
			find := r.FormValue("finder")
			miti := r.FormValue("mitigation")
			expb := r.FormValue("exploitable")
			expl := r.FormValue("exploit")
			exploitable, err := strconv.ParseBool(expb)
			if err != nil {
				logError.Println(err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			cScore, err := strconv.ParseFloat(cvss, 32)
			if err != nil {
				logError.Println(err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			corpscore, err := strconv.ParseFloat(corp, 32)
			if err != nil {
				logError.Println(err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			vuln := varsapi.CreateVulnerability(name, summ, cvsl, test, miti, expl, exploitable, float32(cScore), float32(corpscore))
			finder, err := strconv.Atoi(find)
			if err != nil {
				logError.Println(err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			vuln.Finder = int64(finder)
			vuln.Initiator = user.Emp.ID
			err = varsapi.AddVulnerability(db, vuln)
			if err != nil {
				if varsapi.IsNameNotAvailableError(err) {
					w.WriteHeader(http.StatusNotAcceptable)
					return
				}
				logError.Println(err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			ist := struct {
				ID int64
			}{vuln.ID}
			err = json.NewEncoder(w).Encode(ist)
			if err != nil {
				logError.Println(err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
		} else {
			err := templates.Lookup("notauthorized-get").Execute(w, user)
			if err != nil {
				logError.Printf("Error with templating while performing lookup on %s\n", "notauthorized-get")
				http.Error(w, "Error with templating", http.StatusInternalServerError)
				return
			}
		}
	} else {
		http.Redirect(w, r, "/login", http.StatusFound)
	}
}

func handleVulnerabilityField(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	logRequest(r)
	user, err := getSession(r)
	if err != nil {
		logError.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	v := ps.ByName("vuln")
	vid, err := strconv.Atoi(v)
	if err != nil {
		logError.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	field := ps.ByName("field")
	if user.Authed {
		switch field {
		case "cve":
			cve := ""
			cves, err := varsapi.GetCves(int64(vid))
			if err != nil {
				if !varsapi.IsNoRowsError(err) {
					logError.Println(err)
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
			}
			sort.Strings(*cves)
			cve = strings.Join(*cves, ", ")
			s := struct {
				CVE string
			}{cve}
			err = json.NewEncoder(w).Encode(s)
			if err != nil {
				logError.Println(err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
		default:
			w.WriteHeader(http.StatusNotFound)
			return
		}
	} else {
		http.Redirect(w, r, "/login", http.StatusFound)
	}
}

// handleVulnerabilityPage serves the vulnerability page outline
func handleVulnerabilityPage(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	logRequest(r)
	user, err := getSession(r)
	if err != nil {
		logError.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if user.Authed {
		if user.Emp.Level <= StandardUser {
			s := struct {
				Page string
				User interface{}
			}{"vuln", user}
			w.Header().Add("Content-Type", "text/html")
			err := templates.Lookup("vulns").Execute(w, s)
			if err != nil {
				logError.Printf("Error with templating while performing lookup on %s\n", "vulns")
				http.Error(w, "Error with templating", http.StatusInternalServerError)
				return
			}
		} else {
			err := templates.Lookup("notauthorized-get").Execute(w, user)
			if err != nil {
				logError.Printf("Error with templating while performing lookup on %s\n", "notauthorized-get")
				http.Error(w, "Error with templating", http.StatusInternalServerError)
				return
			}
		}
	} else {
		http.Redirect(w, r, "/login", http.StatusFound)
	}
}

// handleVulnerabilities serves the vulnerability objects
func handleVulnerabilities(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	logRequest(r)
	user, err := getSession(r)
	if err != nil {
		logError.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	v := ps.ByName("vuln")
	if user.Authed {
		if user.Emp.Level <= StandardUser {
			switch v {
			case "all":
				vulns, err := varsapi.GetVulnerabilities()
				if err != nil {
					logError.Println(err)
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				var data []interface{}
				for _, v := range vulns {
					cve := ""
					cves, err := varsapi.GetCves(v.ID)
					if err != nil {
						if !varsapi.IsNoRowsError(err) {
							logError.Println(err)
							w.WriteHeader(http.StatusInternalServerError)
							return
						}
					}
					sort.Strings(*cves)
					cve = strings.Join(*cves, ", ")
					mit := ""
					if v.Dates.Mitigated.Valid {
						mit = v.Dates.Mitigated.Time.Format("Mon, 02 Jan 2006 15:04:05")
					}
					s := struct {
						ID        int64
						Name      string
						Summary   string
						Cvss      float32
						CorpScore float32
						Cve       string
						Initiated string
						Mitigated string
					}{v.ID, v.Name, v.Summary, v.Cvss, v.CorpScore, cve, v.Dates.Initiated.Format("Mon, 02 Jan 2006 15:04:05"), mit}
					data = append(data, s)
				}
				err = json.NewEncoder(w).Encode(data)
				if err != nil {
					logError.Println(err)
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
			case "open":
				vulns, err := varsapi.GetOpenVulnerabilities()
				if err != nil {
					logError.Println(err)
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				var data []interface{}
				for _, v := range vulns {
					cve := ""
					cves, err := varsapi.GetCves(v.ID)
					if err != nil {
						if !varsapi.IsNoRowsError(err) {
							logError.Println(err)
							w.WriteHeader(http.StatusInternalServerError)
							return
						}
					}
					sort.Strings(*cves)
					cve = strings.Join(*cves, ", ")
					s := struct {
						ID        int64
						Name      string
						Summary   string
						Cvss      float32
						CorpScore float32
						Cve       string
						Initiated string
					}{v.ID, v.Name, v.Summary, v.Cvss, v.CorpScore, cve, v.Dates.Initiated.Format("Mon, 02 Jan 2006 15:04:05")}
					data = append(data, s)
				}
				err = json.NewEncoder(w).Encode(data)
				if err != nil {
					logError.Println(err)
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
			case "closed":
				vulns, err := varsapi.GetClosedVulnerabilities()
				if err != nil {
					logError.Println(err)
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				var data []interface{}
				for _, v := range vulns {
					cve := ""
					cves, err := varsapi.GetCves(v.ID)
					if err != nil {
						if !varsapi.IsNoRowsError(err) {
							logError.Println(err)
							w.WriteHeader(http.StatusInternalServerError)
							return
						}
					}
					sort.Strings(*cves)
					cve = strings.Join(*cves, ", ")
					var mit string
					if v.Dates.Mitigated.Valid {
						mit = v.Dates.Mitigated.Time.Format("Mon, 02 Jan 2006 15:04:05")
					} else {
						mit = ""
					}
					s := struct {
						ID        int64
						Name      string
						Summary   string
						Cvss      float32
						CorpScore float32
						Cve       string
						Initiated string
						Mitigated string
					}{v.ID, v.Name, v.Summary, v.Cvss, v.CorpScore, cve, v.Dates.Initiated.Format("Mon, 02 Jan 2006 15:04:05"), mit}
					data = append(data, s)
				}
				err = json.NewEncoder(w).Encode(data)
				if err != nil {
					logError.Println(err)
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
			default:
				vid, err := strconv.Atoi(v)
				if err != nil {
					w.WriteHeader(http.StatusNotFound)
					return
				}
				vuln, err := varsapi.GetVulnerability(int64(vid))
				if err != nil {
					logError.Println(err)
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				err = json.NewEncoder(w).Encode(vuln)
				if err != nil {
					logError.Println(err)
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
			}
		} else {
			err := templates.Lookup("notauthorized-get").Execute(w, user)
			if err != nil {
				logError.Printf("Error with templating while performing lookup on %s\n", "notauthorized-get")
				http.Error(w, "Error with templating", http.StatusInternalServerError)
				return
			}
		}
	} else {
		http.Redirect(w, r, "/login", http.StatusFound)
	}
}

func handleVulnerabilityPut(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	logRequest(r)
	user, err := getSession(r)
	if err != nil {
		logError.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	v := ps.ByName("vuln")
	vid, err := strconv.Atoi(v)
	if err != nil {
		logError.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	field := ps.ByName("field")
	if user.Authed {
		switch field {
		case "cve":
			if user.Emp.Level <= StandardUser {
				cve := r.FormValue("cve")
				err := varsapi.AddCve(db, int64(vid), cve)
				if err != nil {
					logError.Println(err)
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				w.WriteHeader(http.StatusOK)
			} else {
				w.WriteHeader(http.StatusUnauthorized)
			}
		case "ticket":
			if user.Emp.Level <= StandardUser {
				ticket := r.FormValue("ticket")
				err := varsapi.AddTicket(db, int64(vid), ticket)
				if err != nil {
					logError.Println(err)
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				w.WriteHeader(http.StatusOK)
			} else {
				w.WriteHeader(http.StatusUnauthorized)
			}
		case "ref":
			if user.Emp.Level <= StandardUser {
				ref := r.FormValue("ref")
				err := varsapi.AddRef(db, int64(vid), ref)
				if err != nil {
					logError.Println(err)
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				w.WriteHeader(http.StatusOK)
			} else {
				w.WriteHeader(http.StatusUnauthorized)
			}
		case "affected":
			if user.Emp.Level <= StandardUser {
				sys := r.FormValue("system")
				sid, err := strconv.Atoi(sys)
				if err != nil {
					logError.Println(err)
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				err = varsapi.AddAffected(db, int64(vid), int64(sid))
				if err != nil {
					logError.Println(err)
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				w.WriteHeader(http.StatusOK)
			} else {
				w.WriteHeader(http.StatusUnauthorized)
			}
		case "note":
			if user.Emp.Level <= StandardUser {
				note := r.FormValue("note")
				err := varsapi.AddNote(db, int64(vid), user.Emp.ID, note)
				if err != nil {
					logError.Println(err)
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				w.WriteHeader(http.StatusOK)
			} else {
				w.WriteHeader(http.StatusUnauthorized)
			}
		case "mitigated":
			if user.Emp.Level <= PrivilegedUser {
				err := varsapi.CloseVulnerability(db, int64(vid))
				if err != nil {
					logError.Println(err)
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				w.WriteHeader(http.StatusOK)
			} else {
				w.WriteHeader(http.StatusUnauthorized)
			}
		default:
			w.WriteHeader(http.StatusTeapot)
			return
		}
	} else {
		http.Redirect(w, r, "/login", http.StatusFound)
	}
}

func handleVulnerabilityDelete(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	logRequest(r)
	user, err := getSession(r)
	if err != nil {
		logError.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	v := ps.ByName("vuln")
	vid, err := strconv.Atoi(v)
	if err != nil {
		logError.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	field := ps.ByName("field")
	if user.Authed {
		switch field {
		case "cve":
			if user.Emp.Level <= StandardUser {
				cve := ps.ByName("item")
				err := varsapi.DeleteCve(db, int64(vid), cve)
				if err != nil {
					logError.Println(err)
					w.WriteHeader(http.StatusInternalServerError)
					return
				} else {
					w.WriteHeader(http.StatusOK)
				}
			} else {
				w.WriteHeader(http.StatusUnauthorized)
			}
		case "ticket":
			if user.Emp.Level <= StandardUser {
				ticket := ps.ByName("item")
				err := varsapi.DeleteTicket(db, int64(vid), ticket)
				if err != nil {
					logError.Println(err)
					w.WriteHeader(http.StatusInternalServerError)
					return
				} else {
					w.WriteHeader(http.StatusOK)
				}
			} else {
				w.WriteHeader(http.StatusUnauthorized)
			}
		case "ref":
			if user.Emp.Level <= StandardUser {
				b, err := ioutil.ReadAll(r.Body)
				if err != nil {
					logError.Println(err)
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				ref := string(b)
				err = varsapi.DeleteRef(db, int64(vid), ref)
				if err != nil {
					logError.Println(err)
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				w.WriteHeader(http.StatusOK)
			} else {
				w.WriteHeader(http.StatusUnauthorized)
			}
		case "affected":
			if user.Emp.Level <= StandardUser {
				sys := ps.ByName("item")
				sid, err := strconv.Atoi(sys)
				if err != nil {
					logError.Println(err)
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				err = varsapi.DeleteAffected(db, int64(vid), int64(sid))
				if err != nil {
					logError.Println(err)
					w.WriteHeader(http.StatusInternalServerError)
					return
				} else {
					w.WriteHeader(http.StatusOK)
				}
			} else {
				w.WriteHeader(http.StatusUnauthorized)
			}
		case "note":
			n := ps.ByName("item")
			nid, err := strconv.Atoi(n)
			if err != nil {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			author, err := varsapi.GetNoteAuthor(int64(nid))
			if err != nil {
				logError.Println(err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			if user.Emp.Level <= StandardUser && user.Emp.ID == author {
				err = varsapi.DeleteNote(db, int64(nid))
				if err != nil {
					logError.Println(err)
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				w.WriteHeader(http.StatusOK)
			} else {
				w.WriteHeader(http.StatusUnauthorized)
			}
		case "mitigated":
			if user.Emp.Level <= PrivilegedUser {
				err := varsapi.ReopenVulnerability(db, int64(vid))
				if err != nil {
					logError.Println(err)
					w.WriteHeader(http.StatusInternalServerError)
					return
				} else {
					w.WriteHeader(http.StatusOK)
				}
			} else {
				w.WriteHeader(http.StatusUnauthorized)
			}
		case "vuln":
			if user.Emp.Level <= PrivilegedUser {
				err := varsapi.DeleteVulnerability(db, int64(vid))
				if err != nil {
					logError.Println(err)
					w.WriteHeader(http.StatusInternalServerError)
					return
				} else {
					w.WriteHeader(http.StatusOK)
				}
			} else {
				w.WriteHeader(http.StatusUnauthorized)
			}
		default:
			w.WriteHeader(http.StatusTeapot)
			return
		}
	} else {
		http.Redirect(w, r, "/login", http.StatusFound)
	}
}

func handleVulnerabilityPost(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	logRequest(r)
	user, err := getSession(r)
	if err != nil {
		logError.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	v := ps.ByName("vuln")
	vid, err := strconv.Atoi(v)
	if err != nil {
		logError.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	field := ps.ByName("field")
	if user.Authed {
		switch field {
		case "name":
			if user.Emp.Level <= PrivilegedUser {
				name := r.FormValue("name")
				err := varsapi.UpdateVulnerabilityName(db, int64(vid), name)
				if err != nil {
					if varsapi.IsNameNotAvailableError(err) {
						w.WriteHeader(http.StatusNotAcceptable)
						return
					}
					logError.Println(err)
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				w.WriteHeader(http.StatusOK)
				return
			} else {
				w.WriteHeader(http.StatusUnauthorized)
			}
		case "summary":
			if user.Emp.Level <= StandardUser {
				summ := r.FormValue("summary")
				err := varsapi.UpdateVulnerabilitySummary(db, int64(vid), summ)
				if err != nil {
					logError.Println(err)
					w.WriteHeader(http.StatusInternalServerError)
					return
				} else {
					w.WriteHeader(http.StatusOK)
				}
			} else {
				w.WriteHeader(http.StatusUnauthorized)
			}
		case "cve":
			if user.Emp.Level <= StandardUser {
				oldcve := ps.ByName("item")
				cve := r.FormValue("cve")
				err := varsapi.UpdateCve(db, int64(vid), oldcve, cve)
				if err != nil {
					logError.Println(err)
					w.WriteHeader(http.StatusInternalServerError)
					return
				} else {
					w.WriteHeader(http.StatusOK)
				}
			} else {
				w.WriteHeader(http.StatusUnauthorized)
			}
		case "cvss":
			if user.Emp.Level <= StandardUser {
				cvssScore := r.FormValue("cvssScore")
				cvssLink := r.FormValue("cvssLink")
				cScore, err := strconv.ParseFloat(cvssScore, 32)
				if err != nil {
					logError.Println(err)
					w.WriteHeader(http.StatusInternalServerError)
					return
				} else {
					err := varsapi.UpdateCvss(db, int64(vid), float32(cScore), cvssLink)
					if err != nil {
						logError.Println(err)
						w.WriteHeader(http.StatusInternalServerError)
						return
					} else {
						w.WriteHeader(http.StatusOK)
					}
				}
			} else {
				w.WriteHeader(http.StatusUnauthorized)
			}
		case "corpscore":
			if user.Emp.Level <= StandardUser {
				corpscore := r.FormValue("corpscore")
				cScore, err := strconv.ParseFloat(corpscore, 32)
				if err != nil {
					logError.Println(err)
					w.WriteHeader(http.StatusInternalServerError)
					return
				} else {
					err := varsapi.UpdateCorpScore(db, int64(vid), float32(cScore))
					if err != nil {
						logError.Println(err)
						w.WriteHeader(http.StatusInternalServerError)
						return
					} else {
						w.WriteHeader(http.StatusOK)
					}
				}
			} else {
				w.WriteHeader(http.StatusUnauthorized)
			}
		case "finder":
			if user.Emp.Level <= PrivilegedUser {
				finder := r.FormValue("finder")
				eid, err := strconv.Atoi(finder)
				if err != nil {
					logError.Println(err)
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				err = varsapi.UpdateFinder(db, int64(vid), int64(eid))
				if err != nil {
					logError.Println(err)
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				w.WriteHeader(http.StatusOK)
				return
			} else {
				w.WriteHeader(http.StatusUnauthorized)
			}
		case "test":
			if user.Emp.Level <= StandardUser {
				test := r.FormValue("test")
				err := varsapi.UpdateVulnerabilityTest(db, int64(vid), test)
				if err != nil {
					logError.Println(err)
					w.WriteHeader(http.StatusInternalServerError)
					return
				} else {
					w.WriteHeader(http.StatusOK)
				}
			} else {
				w.WriteHeader(http.StatusUnauthorized)
			}
		case "mitigation":
			if user.Emp.Level <= StandardUser {
				mitigation := r.FormValue("mitigation")
				err := varsapi.UpdateVulnerabilityMitigation(db, int64(vid), mitigation)
				if err != nil {
					logError.Println(err)
					w.WriteHeader(http.StatusInternalServerError)
					return
				} else {
					w.WriteHeader(http.StatusOK)
				}
			} else {
				w.WriteHeader(http.StatusUnauthorized)
			}
		case "ticket":
			if user.Emp.Level <= StandardUser {
				oldticket := ps.ByName("item")
				ticket := r.FormValue("ticket")
				err := varsapi.UpdateTicket(db, int64(vid), oldticket, ticket)
				if err != nil {
					logError.Println(err)
					w.WriteHeader(http.StatusInternalServerError)
					return
				} else {
					w.WriteHeader(http.StatusOK)
				}
			} else {
				w.WriteHeader(http.StatusUnauthorized)
			}
		case "ref":
			if user.Emp.Level <= StandardUser {
				oldRef := r.FormValue("oldr")
				newRef := r.FormValue("newr")
				err := varsapi.UpdateReference(db, int64(vid), oldRef, newRef)
				if err != nil {
					logError.Println(err)
					w.WriteHeader(http.StatusInternalServerError)
					return
				} else {
					w.WriteHeader(http.StatusOK)
				}
			} else {
				w.WriteHeader(http.StatusUnauthorized)
			}
		case "exploitable":
			if user.Emp.Level <= StandardUser {
				exploitable := r.FormValue("exploitable")
				b, err := strconv.ParseBool(exploitable)
				if err != nil {
					logError.Println(err)
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				err = varsapi.UpdateExploitable(db, int64(vid), b)
				if err != nil {
					logError.Println(err)
					w.WriteHeader(http.StatusInternalServerError)
					return
				} else {
					w.WriteHeader(http.StatusOK)
				}
			} else {
				w.WriteHeader(http.StatusUnauthorized)
			}
		case "exploit":
			if user.Emp.Level <= StandardUser {
				exploit := r.FormValue("exploit")
				err = varsapi.UpdateExploit(db, int64(vid), exploit)
				if err != nil {
					logError.Println(err)
					w.WriteHeader(http.StatusInternalServerError)
					return
				} else {
					w.WriteHeader(http.StatusOK)
				}
			} else {
				w.WriteHeader(http.StatusUnauthorized)
			}
		case "affected":
			if user.Emp.Level <= StandardUser {
				sys := ps.ByName("item")
				sid, err := strconv.Atoi(sys)
				if err != nil {
					logError.Println(err)
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				patched := r.FormValue("patched")
				b, err := strconv.ParseBool(patched)
				if err != nil {
					logError.Println(err)
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				err = varsapi.UpdateAffected(db, int64(vid), int64(sid), b)
				if err != nil {
					logError.Println(err)
					w.WriteHeader(http.StatusInternalServerError)
					return
				} else {
					w.WriteHeader(http.StatusOK)
				}
			} else {
				w.WriteHeader(http.StatusUnauthorized)
			}
		default:
			w.WriteHeader(http.StatusTeapot)
			return
		}
	}
}

// getSession unpacks the objects from the session cookie associated with the request and returns them.
func getSession(r *http.Request) (*User, error) {
	var user User
	session := sessionManager.Load(r)
	err := session.GetObject("user", &user)
	if err != nil {
		return &user, err
	}
	return &user, nil
}

// logRequest logs the incoming http request including client IP and port.
func logRequest(r *http.Request) {
	reqDump, err := httputil.DumpRequest(r, true)
	if err != nil {
		logError.Printf("Error dumping request: %v\n", err)
	}
	req := fmt.Sprintf("Client IP/Port: %s | Request: %s", r.RemoteAddr, string(reqDump))
	req = strings.Replace(req, "\r\n", " | ", -1)
	logInfo.Println(cookRegex.ReplaceAllString(req, "session=****"))
}
