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

## Configuration

Follow the [Strava API setup guide](https://developers.strava.com/docs/getting-started/). Grab the resulting client ID, client secret, and JSON token blob. Then:

```sh
$ mkdir ~/.sls
$ cat <<EOF > ~/.sls/config.toml
athlete_id = <id>
client_id = <client ID>
client_secret = "<client secret>"
page_hint = 10
EOF
```

Paste the JSON token blob into `~/.sls/token`. The config file and token probably shouldn't be world readable.

`page_hint` is the only notable config option. `sls` fetches `page_hint` pages of activities in parallel then fetches any remaining pages of data linearly. Ideally `sls` could figure out how many pages to fetch in parallel on its own but as far as I can tell the Strava API has no way to get the total number of activities. [`getStats`](https://developers.strava.com/docs/reference/#api-Athletes-getStats) returns the number of bike/run/swim activities but these numbers won't include any other activity type. Therefore, you should set `page_hint` to something greater than your total activity count divided by `per_page` (which defaults to 100).
