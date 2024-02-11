package main

import (
	"html/template"
	"io"
	"log"
	"os"

	"github.com/joho/godotenv"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

type Template struct {
	tmpl *template.Template
}

func NewTemplate(parse string) *Template {
	return &Template{
		tmpl: template.Must(template.ParseGlob(parse)),
	}
}

func (t *Template) Render(w io.Writer, name string, data interface{}, c echo.Context) error {
	return t.tmpl.ExecuteTemplate(w, name, data)
}

var TurnUrl string
var TurnUser string
var TurnCred string

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	TurnUrl = os.Getenv("TURN_URL")
	TurnUser = os.Getenv("TURN_USERNAME")
	TurnCred = os.Getenv("TURN_CRED")

	e := echo.New()

	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	e.Renderer = NewTemplate("./views/*.html")
	e.Static("/", "./assets")

	e.GET("/", HandleHome)
	e.GET("/room/create", HandleCreate)
	e.GET("/room/:id", HandleRoom)
	e.GET("/room/:id/rws", HandleRoomWS)

	e.Logger.Fatal(e.Start(":5000"))
}
