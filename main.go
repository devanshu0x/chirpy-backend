package main

import (
	"net/http"
	"fmt"
)

func  main(){
	mux:= http.NewServeMux()
	mux.Handle("/app/",http.StripPrefix("/app",http.FileServer(http.Dir("."))))
	server:=http.Server{
		Handler: mux,
		Addr: ":8080",
	}
	mux.HandleFunc("/healthz", func (w http.ResponseWriter, req *http.Request){
		w.Header().Add("content-type","text/plain; charset=utf-8")
		w.WriteHeader(200)

		w.Write([]byte("OK"))
	})
	err:=server.ListenAndServe()
	if err!=nil{
		fmt.Printf("Error in starting sever: %v",err)
	}

}