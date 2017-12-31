# gmail-scraper
An application that will index your gmail account into elasticsearch, and record metrics into [Prometheus](https://prometheus.io/).
NOTE: This is just a demo app, you will definitely want a more unique code than "woot" when verifying oauth responses.

### Installation
* Setup a GMail Oauth client by following the instructions [here](https://developers.google.com/identity/protocols/OAuth2).
* Export your client id and client secret
  `$ export GOOGLE_CLIENT_ID=<your-client-id.apps.googleusercontent.com>`
  `$ export GOOGLE_CLIENT_SECRET=<your-client-secret>`
* run `$ docker-compose up`
* Navigate to http://localhost:8080
* Click "Index GMail Account" and allow our client
* After clicking "Allow" the indexing will begin and may take awhile depending on how many emails are in your account

### Viewing metrics

You can access Prometheus at http://localhost:9090 and Grafana at http://localhost:3000 in order to setup graphs and dashboards
to see some metrics. All elasticsearch metrics are exported using 
[justwatchcom/elasticsearch_exported](https://github.com/justwatchcom/elasticsearch_exporter), and some interesting histograms
gmail-scraper exports are:
* `email_scraper_get_email_seconds_bucket` - how long it takes to retrieve an email from GMail
* `email_scraper_get_id_seconds_bucket` - how long it takes to retrieve a page (100) email ids from GMail
* `email_scraper_index_email_seconds_bucket` - how long it takes to index a document into elasticsearch
* Since these are histograms, instead of `_bucket` there are also 
  * `_count` to view number of requests
  * `_sum` to view
  total time spent doing each operation
