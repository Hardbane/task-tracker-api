#!/usr/bin/env bash
set -euo pipefail

BASE_URL=${BASE_URL:-http://localhost:8080}

curl -s -X POST "$BASE_URL/api/v1/register" \
  -H 'Content-Type: application/json' \
  -d '{"name":"Mikhail","email":"mikhail@example.com","password":"password123"}' | jq .

TOKEN=$(curl -s -X POST "$BASE_URL/api/v1/login" \
  -H 'Content-Type: application/json' \
  -d '{"email":"mikhail@example.com","password":"password123"}' | jq -r .token)

curl -s -X POST "$BASE_URL/api/v1/teams" \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"name":"Backend Team"}' | jq .

curl -s -X POST "$BASE_URL/api/v1/tasks" \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"team_id":1,"title":"First task","description":"Prepare Go API","status":"todo"}' | jq .

curl -s "$BASE_URL/api/v1/tasks?team_id=1&page=1&limit=20" \
  -H "Authorization: Bearer $TOKEN" | jq .

curl -s "$BASE_URL/api/v1/reports/team-summary" \
  -H "Authorization: Bearer $TOKEN" | jq .
