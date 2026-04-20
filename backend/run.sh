#!/bin/bash

docker build -t aichat-backend:latest .

docker-compose up --build -d