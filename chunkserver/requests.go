package main

type RegisterRequest struct {
	Port string `json:"port"`
}

type HeartbeatRequest struct {
	Port string `json:"port"`
}
