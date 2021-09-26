# sls

`ls` for Strava activities. Makes finding old things easier.

```sh
$ sls
#     Date       ID  Type   Dist   Elev  Gear   Name
2010-06-04  2063782  Ride   85.5   1478  Focus  06/04/2010 Megève, Rhône-Alpes, France
2010-06-05  2065048  Ride  166.5   3555  Focus  Megeve sportive warm-up
2010-06-06  2065038  Ride  114.0   2821  Focus  Time Megeve sportive
2010-06-07  2065018  Ride   58.8   1507  Focus  06/07/2010 Megève, Rhône-Alpes, France
[..]
```

If you're an [fzf](https://github.com/junegunn/fzf) fan this shortcut will allow you to select multiple activities and open them in your browser:

```sh
$ type -f strava
strava () {
    sls \
    | fzf --multi --no-sort --tac --header-lines=1 \
    | awk '{ print "https://www.strava.com/activities/" $2 }' \
    | xargs open
}
```

Demo of `sls` + `fzf` (click for video):

[![asciicast](https://asciinema.org/a/428385.png)](https://asciinema.org/a/428385)

Another use: tracking how many kilometers a chain has:

```sh
$ type -f chain-dist
chain-dist () {
    id=$(cat ~/.chain-id)
    sls -j \
    | jq -r '.[] | "\(.activity.id)\t\(.activity.type)\t\(.activity.external_id)\t\(.gear.name)\t\(.activity.distance)"' \
    | awk -v id=$id '
        $1 == id {
            go = 1
        }
        {
            if (go && $4 == "R3") {
                if ($2 == "VirtualRide" || $3 ~ /^(trainerroad|zwift)/) {
                    vkm += $5
                }
                km += $5
                n++
            }
        }
        END {
            printf("%d rides, %d km (%d km indoors)\n", n, km / 1000, vkm / 1000)
        }'
}

$ echo 3179772474 > ~/.chain-id  # first activity on new chain

$ chain-dist
64 rides, 2587 km (1797 km indoors)
```

## Building

```sh
$ git clone git@github.com:markdrayton/sls.git
$ cd sls/cmd/sls
$ go build
```

## Configuration

Follow the [Strava API setup guide](https://developers.strava.com/docs/getting-started/). Make sure you request the `read` and `activity:read_all` scopes to see all of your activities. Grab the resulting client ID, client secret, and JSON token blob. Then:

```sh
$ mkdir ~/.sls
$ cat <<EOF > ~/.sls/config.toml
athlete_id = <your athlete ID>
client_id = <client ID>
client_secret = "<client secret>"
EOF
```

Paste the JSON token blob into `~/.sls/token`. The config file and token probably shouldn't be world readable.

Without an existing cache `sls` will fetch activities in parallel. Once a cache is present it will only fetch activities that have occurred since the latest cached activity. The cache is never automatically dropped so any changes made to cached activities won't be locally reflected. Use `sls -r` to force a cache refresh.

The Strava API doesn't return geocoded start locations (for `sls -s`). `sls` can use the Google Maps API for this purpose by setting a valid `google_maps_api_key` in `config.toml`. To reduce the number of calls to the geocoding API start lat/lng values are rounded to 2km boundaries and the geocoded locations are cached in `~/.sls`.