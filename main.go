package main

import (
	//"github.com/hashicorp/golang-lru"
	"context"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v4"
	"github.com/nats-io/stan.go"
	"html/template"
	"io"
	"log"
	"net/http"
)
func main() {
	
	cache := make(map[string]Client)

	var (
		name     = "postgres"
		password = "postgres"
	)
	
	dbURL := fmt.Sprintf("postgres://%s:%s@localhost:5434/postgres?sslmode=disable", name, password)
	conn, err := pgx.Connect(context.Background(), dbURL)
	if err != nil {
		log.Printf("Не получилось подключиться к базе данных:%v\n", err)
	} else {
		log.Println("Подключение к БД успешно!")
	}
	defer conn.Close(context.Background())
	rows, err := conn.Query(context.Background(), "SELECT * FROM task_l0")
	if err != nil {
		log.Println(err)
	}
	for rows.Next() {
		var (
			bytes []byte
			data  Client
		)
		err = rows.Scan(&bytes)
		if err != nil {
			log.Println(err)
		}

		err = json.Unmarshal(bytes, &data)
		if err != nil {
			log.Println(err)
		}
		cache[data.OrderUid] = data
		log.Printf("Получены данные из бдю Order UID: %s\n", data.OrderUid)
	}
	rows.Close()

	sc, err := stan.Connect("test-cluster", "Nosir", stan.NatsURL("nats://localhost:4222"))
	if err != nil && err != io.EOF {
		log.Fatalln(err)
	} else {
		log.Println("Подключение к Nats-streaming успешно!")
	}
	defer sc.Close()

	_, err = sc.Subscribe("foo1", func(m *stan.Msg) {
		var d Client

		err := json.Unmarshal(m.Data, &d)
		if err != nil {
			log.Println(err)
		} else {
			log.Println(" Данные из NATS-streaming получены!")
		}
		if _, ok := cache[d.OrderUid]; !ok {
			cache[d.OrderUid] = d

			_, err = conn.Exec(context.Background(), "insert into task_l0 values ($1, $2)", d.OrderUid, m.Data)
			if err != nil {
				log.Println(err)
			} else {
				log.Printf("Добавлены в БД Order UID: %s\n", d.OrderUid)
			}
		}
	}, stan.StartWithLastReceived())
	if err != nil {
		log.Println(err)
	}
	if err != nil {
		log.Println(err)
	}
	http.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		if req.Method=="GET"{
			tmpl, err := template.ParseFiles("web.html")
			if err != nil {
				http.Error(w, err.Error(), 400)
				return
			}

			err = tmpl.Execute(w, nil)
			if err != nil {
				http.Error(w, err.Error(), 400)
				return
			}
		}

			if req.Method=="POST"{
			if val, ok := cache[req.PostFormValue("order_uid")]; ok {

				b, err := json.MarshalIndent(val, "", "\t")
				if err != nil {
					log.Println(err)
				}
				log.Printf("Отправлены данные Order UID: %s\n", req.PostFormValue("order_uid"))
				fmt.Fprint(w, string(b))
			} else {
				log.Println("Данного Order UID не существует")
				fmt.Fprint(w, "Данные с таким Order UID отсутствует")
			}
		}
	})

	log.Fatal(http.ListenAndServe(":5555", nil))

}
