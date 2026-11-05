#!/usr/bin/env bash
# netmeasure.sh - measure the network path used by actions/download-artifact.
#
#   netmeasure.sh probe   active, time-boxed probes against the real artifact blob URL plus controls
#   netmeasure.sh start   begin background sampling (NIC rx bytes @1s, TCP stats via ss @5s)
#   netmeasure.sh stop    stop sampling and print/emit the summary for the download window
#
# Everything is best-effort and never fails the job. No signed URL or token is ever printed.
#
# Inputs (env):
#   GITHUB_TOKEN, GITHUB_REPOSITORY, GITHUB_RUN_ID   used to resolve the artifact's signed blob URL
#   ARTIFACT_NAME      artifact to probe             (default: talos-artifacts)
#   PROBE_URL          skip resolution, probe this URL instead (testing)
#   PROBE_SECS         cap per probe request          (default: 30)
#   NETMEASURE_DIR     scratch/output dir             (default: $RUNNER_TEMP/netmeasure)

set -uo pipefail

OUT="${NETMEASURE_DIR:-${RUNNER_TEMP:-/tmp}/netmeasure}"
PROBE_SECS="${PROBE_SECS:-30}"
ARTIFACT_NAME="${ARTIFACT_NAME:-talos-artifacts}"
API="${GITHUB_API_URL:-https://api.github.com}"
mkdir -p "$OUT"

now() { date +%s.%N; }
iso() { date -u +%FT%T.%3NZ; }
log() { printf '%s %s\n' "$(iso)" "$*"; }
mbps() { awk -v b="${1:-0}" -v s="${2:-0}" 'BEGIN{ if (s>0) printf "%.2f", b/s/1e6; else print "0.00" }'; }
summary() { [ -n "${GITHUB_STEP_SUMMARY:-}" ] && printf '%s\n' "$*" >>"$GITHUB_STEP_SUMMARY"; return 0; }

# curl -w format: one tab-separated record per request, units are seconds / bytes.
CW='%{http_code}\t%{remote_ip}\t%{time_namelookup}\t%{time_connect}\t%{time_appconnect}\t%{time_starttransfer}\t%{time_total}\t%{size_download}'

# probe_one LABEL URL [curl args...]  -> appends a row to probe.tsv and logs it
probe_one() {
  local label="$1" url="$2"; shift 2
  local rec
  rec=$(curl -sS -o /dev/null -m "$PROBE_SECS" -w "$CW" "$@" "$url" 2>"$OUT/curl.err") || true
  [ -n "$rec" ] || rec=$'000\t-\t0\t0\t0\t0\t0\t0'
  IFS=$'\t' read -r code ip dns conn tls ttfb total bytes <<<"$rec"
  # rate excludes connection setup + time to first byte, i.e. pure transfer speed
  local xfer rate
  xfer=$(awk -v t="$total" -v f="$ttfb" 'BEGIN{ d=t-f; if (d<=0) d=t; printf "%.3f", d }')
  rate=$(mbps "$bytes" "$xfer")
  printf '%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n' "$label" "$code" "$ip" "$dns" "$conn" "$tls" "$ttfb" "$total" "$bytes" "$rate" >>"$OUT/probe.tsv"
  log "probe[$label] http=$code ip=$ip dns=${dns}s connect=${conn}s tls=${tls}s ttfb=${ttfb}s total=${total}s bytes=$bytes xfer_rate=${rate}MB/s"
  [ -s "$OUT/curl.err" ] && log "probe[$label] curl: $(head -c 200 "$OUT/curl.err")"
  return 0
}

# probe_parallel LABEL URL N CHUNK_BYTES -> N concurrent range requests, aggregate over wall clock
probe_parallel() {
  local label="$1" url="$2" n="$3" chunk="$4" i a b t0 t1 sum=0 b_i
  rm -f "$OUT"/par.*
  t0=$(now)
  for ((i = 0; i < n; i++)); do
    a=$((i * chunk)); b=$((a + chunk - 1))
    ( curl -sS -o /dev/null -m "$PROBE_SECS" -r "$a-$b" -w '%{size_download}' "$url" >"$OUT/par.$i" 2>/dev/null || true ) &
  done
  wait
  t1=$(now)
  for ((i = 0; i < n; i++)); do b_i=$(cat "$OUT/par.$i" 2>/dev/null || echo 0); sum=$((sum + ${b_i:-0})); done
  local wall rate
  wall=$(awk -v a="$t0" -v b="$t1" 'BEGIN{printf "%.3f", b-a}')
  rate=$(mbps "$sum" "$wall")
  printf '%s\t-\t-\t0\t0\t0\t0\t%s\t%s\t%s\n' "$label" "$wall" "$sum" "$rate" >>"$OUT/probe.tsv"
  log "probe[$label] streams=$n bytes=$sum wall=${wall}s aggregate_rate=${rate}MB/s"
}

resolve_artifact_url() {
  [ -n "${PROBE_URL:-}" ] && { echo "$PROBE_URL"; return; }
  if [ -z "${GITHUB_TOKEN:-}" ] || [ -z "${GITHUB_REPOSITORY:-}" ] || [ -z "${GITHUB_RUN_ID:-}" ]; then
    log "cannot resolve artifact URL: GITHUB_TOKEN/GITHUB_REPOSITORY/GITHUB_RUN_ID not set" >&2; return
  fi
  local meta id size loc
  meta=$(curl -fsS -m 20 -H "Authorization: Bearer $GITHUB_TOKEN" -H 'Accept: application/vnd.github+json' \
    "$API/repos/$GITHUB_REPOSITORY/actions/runs/$GITHUB_RUN_ID/artifacts?name=$ARTIFACT_NAME&per_page=1") || { log "artifact list failed" >&2; return; }
  id=$(grep -oE '"id": *[0-9]+' <<<"$meta" | head -1 | grep -oE '[0-9]+')
  size=$(grep -oE '"size_in_bytes": *[0-9]+' <<<"$meta" | head -1 | grep -oE '[0-9]+')
  [ -n "$id" ] || { log "artifact '$ARTIFACT_NAME' not found in run" >&2; return; }
  echo "${size:-0}" >"$OUT/artifact_size"
  log "artifact '$ARTIFACT_NAME' id=$id size=${size:-?} bytes" >&2
  # the /zip endpoint answers 302 -> signed blob URL; don't follow it here
  loc=$(curl -sS -m 20 -o /dev/null -D - -H "Authorization: Bearer $GITHUB_TOKEN" \
    "$API/repos/$GITHUB_REPOSITORY/actions/artifacts/$id/zip" | awk 'tolower($1)=="location:"{print $2}' | tr -d '\r')
  [ -n "$loc" ] && echo "$loc" || log "no redirect Location from artifact zip endpoint" >&2
}

cmd_probe() {
  : >"$OUT/probe.tsv"
  log "=== netmeasure probe ($(hostname)) ==="
  log "curl: $(curl --version | head -1)"
  log "egress ip (as seen externally): $(curl -sS -m 10 https://checkip.amazonaws.com 2>/dev/null | tr -d '\n' || echo unknown)"
  log "pod ip: $(hostname -i 2>/dev/null | awk '{print $1}')  iface: $(ip route show default 2>/dev/null | awk '{print $5; exit}')"
  log "proxy env: $(env | grep -i -E '^(https?|all|no)_proxy=' | sed -E 's#//[^@/]*@#//***@#' | tr '\n' ' ')"

  local url host
  url=$(resolve_artifact_url)
  if [ -n "$url" ]; then
    host=$(sed -E 's#^https?://([^/?]+).*#\1#' <<<"$url")
    log "artifact blob host: $host  (resolves: $(getent ahosts "$host" | awk '{print $1}' | sort -u | tr '\n' ' '))"
    probe_one "artifact-first-1KiB" "$url" -r 0-1023
    probe_one "artifact-1-stream" "$url"
    probe_parallel "artifact-4-streams" "$url" 4 $((64 * 1024 * 1024))
    probe_parallel "artifact-8-streams" "$url" 8 $((32 * 1024 * 1024))
  else
    log "no artifact URL; skipping artifact probes"
  fi
  # Controls: another Azure storage account, GitHub release CDN, and a non-GitHub/non-Azure host.
  probe_one "control-azure-blob-other-account" "https://azcliprod.blob.core.windows.net/msi/azure-cli-2.67.0-x64.msi"
  probe_one "control-github-release-cdn" "https://github.com/siderolabs/talos/releases/download/v1.14.5/talosctl-linux-amd64" -L
  probe_one "control-non-github" "https://deb.debian.org/debian/dists/stable/main/Contents-amd64.gz" -L

  summary "### netmeasure: pre-download probes (cap ${PROBE_SECS}s each)"
  summary "| probe | http | remote ip | dns s | connect s | tls s | ttfb s | total s | bytes | MB/s (transfer) |"
  summary "|---|---|---|---|---|---|---|---|---|---|"
  awk -F'\t' '{printf "| %s | %s | %s | %s | %s | %s | %s | %s | %s | **%s** |\n",$1,$2,$3,$4,$5,$6,$7,$8,$9,$10}' "$OUT/probe.tsv" |
    while IFS= read -r l; do summary "$l"; done
  log "=== end probe ==="
}

cmd_start() {
  local iface
  iface=$(ip route show default 2>/dev/null | awk '{print $5; exit}')
  [ -n "$iface" ] || { log "no default-route interface; sampler not started"; return 0; }
  : >"$OUT/rx.log"; : >"$OUT/ss.log"
  now >"$OUT/dl_start"
  echo "$iface" >"$OUT/iface"
  # setsid+nohup so the sampler survives the step boundary; stopped explicitly by `stop`
  NM_OUT="$OUT" NM_IFACE="$iface" setsid nohup bash -c '
    n=0
    while :; do
      ts=$(date +%s.%N)
      rx=$(awk -F"[: ]+" -v i="$NM_IFACE" "\$2==i{print \$3}" /proc/net/dev)
      echo "$ts $rx" >>"$NM_OUT/rx.log"
      if [ $((n % 5)) -eq 0 ] && command -v ss >/dev/null; then
        { echo "T $ts"; ss -tinH state established "( dport = :443 )"; } >>"$NM_OUT/ss.log" 2>/dev/null
      fi
      n=$((n + 1)); sleep 1
    done' >/dev/null 2>&1 &
  echo $! >"$OUT/sampler.pid"
  log "sampler started pid=$(cat "$OUT/sampler.pid") iface=$iface"
}

cmd_stop() {
  now >"$OUT/dl_end"
  [ -f "$OUT/sampler.pid" ] && kill "$(cat "$OUT/sampler.pid")" 2>/dev/null
  [ -s "$OUT/rx.log" ] || { log "no samples collected"; return 0; }
  local t0 t1 dur size
  t0=$(cat "$OUT/dl_start"); t1=$(cat "$OUT/dl_end")
  dur=$(awk -v a="$t0" -v b="$t1" 'BEGIN{printf "%.1f", b-a}')
  size=$(cat "$OUT/artifact_size" 2>/dev/null || echo 0)

  # per-interval rx rate (MB/s) within the download window
  awk -v t0="$t0" -v t1="$t1" 'NR>1 && $1>=t0 && $1<=t1+1 && $1>pt { printf "%.3f %.4f\n", $1, ($2-pb)/($1-pt)/1e6 } { pt=$1; pb=$2 }' "$OUT/rx.log" >"$OUT/rate.log"
  local n total
  n=$(wc -l <"$OUT/rate.log")
  total=$(awk -v t0="$t0" -v t1="$t1" '$1>=t0 && $1<=t1+1 { if (!f) { f=$2 } l=$2 } END{ print l-f }' "$OUT/rx.log")

  log "=== netmeasure download window: ${dur}s, ${n} 1s-samples, rx_bytes=${total} ==="
  [ "$size" -gt 0 ] 2>/dev/null && log "artifact size=${size}B  effective throughput (size/window)=$(mbps "$size" "$dur") MB/s  nic-rx/size ratio=$(awk -v r="$total" -v s="$size" 'BEGIN{printf "%.2f", r/s}')"
  log "--- 10s-bucket timeline (MB/s avg) ---"
  awk -v t0="$t0" '{ b=int(($1-t0)/10); s[b]+=$2; c[b]++ } END{ for (b=0;b<=int(NR/10)+1;b++) if (c[b]) printf "t+%4ds  %.2f MB/s\n", b*10, s[b]/c[b] }' "$OUT/rate.log"

  # distribution, stalls
  local stats
  stats=$(sort -g -k2 "$OUT/rate.log" | awk '{v[NR]=$2} END{ if(!NR){print "n/a"; exit}
      printf "min=%.2f p10=%.2f p50=%.2f p90=%.2f max=%.2f", v[1], v[int(NR*0.1)+1], v[int(NR*0.5)+1], v[int(NR*0.9)+1], v[NR] }')
  local slow longest
  slow=$(awk '$2<0.25{c++} END{print c+0}' "$OUT/rate.log")
  longest=$(awk '{ if ($2<0.25) { r++; if (r>m) m=r } else r=0 } END{print m+0}' "$OUT/rate.log")
  log "rate MB/s: $stats ; seconds below 0.25MB/s: $slow ; longest slow run: ${longest}s"

  # TCP-level evidence: loss/retransmits, RTT, congestion window on :443 connections
  local ssline=""
  if [ -s "$OUT/ss.log" ]; then
    local maxret maxrtt medrtt mincw
    maxret=$(grep -oE 'retrans:[0-9]+/[0-9]+' "$OUT/ss.log" | awk -F'[:/]' '{ if ($3>m) m=$3 } END{print m+0}')
    maxrtt=$(grep -oE ' rtt:[0-9.]+' "$OUT/ss.log" | awk -F: '{ if ($2>m) m=$2 } END{print m+0}')
    medrtt=$(grep -oE ' rtt:[0-9.]+' "$OUT/ss.log" | awk -F: '{print $2}' | sort -g | awk '{v[NR]=$1} END{print (NR? v[int(NR/2)+1] : 0)}')
    mincw=$(grep -oE ' cwnd:[0-9]+' "$OUT/ss.log" | awk -F: 'NR==1||$2<m{m=$2} END{print m+0}')
    ssline="tcp(:443, all conns): max_retrans_total=$maxret rtt_ms median=$medrtt max=$maxrtt min_cwnd=$mincw"
    log "$ssline"
    log "--- ss snapshot with most bytes_received (last) ---"
    grep -E 'bytes_received:' "$OUT/ss.log" | awk '{ for(i=1;i<=NF;i++) if ($i ~ /^bytes_received:/) { split($i,a,":"); if (a[2]>=m) { m=a[2]; l=$0 } } } END{print l}' | cut -c1-600
  else
    log "ss not available; no TCP-level stats"
  fi

  summary ""
  summary "### netmeasure: download window"
  summary "- window: **${dur}s**, nic rx: **${total} B**$([ "$size" -gt 0 ] 2>/dev/null && echo ", artifact: ${size} B, effective: **$(mbps "$size" "$dur") MB/s**")"
  summary "- rate MB/s: ${stats}; seconds <0.25MB/s: **${slow}** (longest run ${longest}s)"
  [ -n "$ssline" ] && summary "- ${ssline}"
  log "=== end netmeasure ==="
}

case "${1:-}" in
  probe) cmd_probe ;;
  start) cmd_start ;;
  stop)  cmd_stop ;;
  *) echo "usage: $0 probe|start|stop" >&2; exit 2 ;;
esac
exit 0
