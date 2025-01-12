#!/bin/bash
export $(grep -v '^#' secrets.env | xargs)
go build && ./gofinbot
