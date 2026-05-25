package main

import (
	"net/http"
	"fmt"
)

func  main(){
	mux:= http.NewServeMux()
	server:=http.Server{
		Handler: mux,
		Addr: ":8080",
	}
	err:=server.ListenAndServe()
	if err!=nil{
		fmt.Printf("Error in starting sever: %v",err)
	}

}