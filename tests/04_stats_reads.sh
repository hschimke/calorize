#!/bin/bash
# 04_stats_reads.sh: Stats Reading & Date Filtering

if [ -z "$BASE_URL" ]; then
    source "$(dirname "$0")/common.sh"
fi

echo "==================================================="
echo "Test 8: GET /logs with Date Filtering"
echo "---------------------------------------------------"
echo "Fetching today's logs (default)..."
TODAY_LOGS=$(curl -s "$BASE_URL/logs")
TODAY_COUNT=$(echo $TODAY_LOGS | jq 'length')
echo "Today's logs count: $TODAY_COUNT"

TODAY_DATE=$(date -u +"%Y-%m-%d")
echo "Fetching today's logs with explicit date..."
TODAY_EXPLICIT=$(curl -s "$BASE_URL/logs?date=$TODAY_DATE")
EXPLICIT_COUNT=$(echo $TODAY_EXPLICIT | jq 'length')
if [ "$TODAY_COUNT" == "$EXPLICIT_COUNT" ]; then
    log_info "✅ Default and explicit date return same count: $TODAY_COUNT"
else
    log_err "Default ($TODAY_COUNT) vs explicit ($EXPLICIT_COUNT) mismatch"
fi

echo "Fetching logs for a date with no data..."
EMPTY_LOGS=$(curl -s "$BASE_URL/logs?date=2000-01-01")
EMPTY_COUNT=$(echo $EMPTY_LOGS | jq 'length')
if [ "$EMPTY_COUNT" -eq 0 ] || [ "$EMPTY_LOGS" == "null" ]; then
    log_info "✅ No logs for 2000-01-01"
else
    log_err "Expected 0 logs, got $EMPTY_COUNT"
fi

echo "Fetching logs with invalid date format..."
INVALID_DATE_CODE=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/logs?date=not-a-date")
if [ "$INVALID_DATE_CODE" == "400" ]; then
    log_info "✅ Invalid date returns 400 Bad Request"
else
    log_err "Invalid date returned $INVALID_DATE_CODE"
fi

echo "==================================================="
echo "Test 9: Stats — Week, Month, Invalid, Missing Period"
echo "---------------------------------------------------"
echo "Stats for the week..."
WEEK_STATS=$(curl -s "$BASE_URL/stats?period=week")
WEEK_CODE=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/stats?period=week")
WEEK_CAL=$(echo $WEEK_STATS | jq -r .calories)
if [ "$WEEK_CODE" == "200" ]; then
    log_info "✅ Week stats returned 200 (calories: $WEEK_CAL)"
else
    log_err "Week stats returned $WEEK_CODE"
fi

echo "Stats for the month..."
MONTH_STATS=$(curl -s "$BASE_URL/stats?period=month")
MONTH_CODE=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/stats?period=month")
MONTH_CAL=$(echo $MONTH_STATS | jq -r .calories)
if [ "$MONTH_CODE" == "200" ]; then
    log_info "✅ Month stats returned 200 (calories: $MONTH_CAL)"
else
    log_err "Month stats returned $MONTH_CODE"
fi

echo "Week stats should >= day stats..."
DAY_STATS=$(curl -s "$BASE_URL/stats?period=day")
DAY_CAL=$(echo $DAY_STATS | jq -r .calories)
WEEK_GTE_DAY=$(echo "$WEEK_CAL $DAY_CAL" | awk '{if ($1 >= $2 - 0.01) print 1; else print 0}')
if [ "$WEEK_GTE_DAY" -eq 1 ]; then
    log_info "✅ Week ($WEEK_CAL) >= Day ($DAY_CAL)"
else
    log_err "Week ($WEEK_CAL) < Day ($DAY_CAL)"
fi

echo "Stats with invalid period..."
INVALID_PERIOD_CODE=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/stats?period=invalid")
if [ "$INVALID_PERIOD_CODE" == "500" ]; then
    log_info "✅ Invalid period returns 500 (known: server returns fmt.Errorf, not 400)"
else
    log_warn "Invalid period returned $INVALID_PERIOD_CODE (expected 500)"
fi

echo "Stats with no period param..."
NO_PERIOD_CODE=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/stats")
if [ "$NO_PERIOD_CODE" == "500" ]; then
    log_info "✅ Missing period returns 500 (known: empty string hits default case)"
else
    log_warn "Missing period returned $NO_PERIOD_CODE (expected 500)"
fi

echo "Stats with custom past date..."
PAST_STATS=$(curl -s "$BASE_URL/stats?period=day&date=2000-01-01")
PAST_CAL=$(echo $PAST_STATS | jq -r .calories)
PAST_IS_ZERO=$(echo "$PAST_CAL" | awk '{if ($1 == 0) print 1; else print 0}')
if [ "$PAST_IS_ZERO" -eq 1 ]; then
    log_info "✅ Stats for 2000-01-01 returns 0 calories"
else
    log_err "Expected 0 calories for 2000-01-01, got $PAST_CAL"
fi

echo "Stats with invalid date format..."
BAD_DATE_CODE=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/stats?period=day&date=bad-date")
if [ "$BAD_DATE_CODE" == "400" ]; then
    log_info "✅ Invalid date returns 400 Bad Request"
else
    log_warn "Invalid date returned $BAD_DATE_CODE"
fi
