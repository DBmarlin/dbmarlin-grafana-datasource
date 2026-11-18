# DBmarlin data source for Grafana

[DBmarlin](https://www.dbmarlin.com) monitors the performance of your databases (Oracle, MySQL, PostgreSQL, SQL Server, Db2, Informix, SAP HANA, Snowflake and MongoDB) and shows where the time goes: which SQL, which waits, which sessions and users. This plugin brings that data into Grafana, so DBmarlin's database performance can sit on the same dashboards, and alert alongside, the rest of your metrics.

Every graph and table can link back to the matching statement or activity view in DBmarlin for a deeper look.

## What you can do

- **Activity over time.** Chart database time as average active sessions (AAS), wait time, wait count or average wait time, for one instance, or for several instances either as separate series or added together. Optionally group by wait event, SQL, session, user, database, client, program or module, showing the top N values and everything else as "Other", as DBmarlin does.
- **Top N tables.** List the biggest consumers of database time by wait event, SQL, session, user, database, client, program or module, with wait time, share of the total, AAS, wait count and average wait, and an "Open in DBmarlin" link on each row.
- **SQL statistics.** Rank SQL statements by any statistic DBmarlin collects for the database type (duration, executions, reads and so on), with extra columns of your choice.
- **Database statistics and host metrics.** Chart the statistics DBmarlin collects for each database type, and host CPU, memory, I/O and disk utilisation.
- **Annotations.** Overlay DBmarlin alerts, changes and custom events, such as deployments, on any panel. Events cover the selected instances and their hosts.
- **Dashboard variables.** Create an instance variable, then use it in queries to build one dashboard that works for any instance.
- **Alerting.** Queries work with Grafana alert rules.
- **Ready-made dashboards.** Import *DBmarlin - Instance overview* and *DBmarlin - Fleet overview* from the **Dashboards** tab of the data source settings.

## Requirements

- Grafana 11.0 or later.
- DBmarlin 6.11 or later, on a server that the **Grafana server** can reach (not just your browser). The plugin has its own version number, independent of the DBmarlin version.
- A DBmarlin user for the plugin. Everyone who can query the data source sees what that user sees, so use a dedicated user with the access you want to expose, such as read-only access limited by tags. If DBmarlin has no authentication, leave the username empty.
- The plugin includes a backend for Linux (x86-64 and ARM64), Windows and macOS (Apple silicon).

## Installation

Install the plugin from the Grafana plugin catalog: in Grafana, go to **Administration > Plugins and data > Plugins**, search for **DBmarlin** and click **Install**. On a self-managed Grafana you can also run `grafana cli plugins install dbmarlin-dbmarlin-datasource` and restart Grafana.

## Getting started

1. Go to **Connections > Data sources > Add new data source** and choose **DBmarlin**.
2. Enter the **URL** of DBmarlin, for example `https://dbmarlin.example.com:9090`, and the user's **username** and **password**.
3. Click **Save & test**. Grafana reports how many instances the user can see.
4. Create a panel, choose the DBmarlin data source, pick a **Query type** and select one or more instances.

Other settings, all optional:

| Setting | Use |
| --- | --- |
| Skip TLS verify, TLS CA certificate | For DBmarlin servers with a self-signed or private-CA certificate. |
| UI URL | The address your browser uses for DBmarlin, if it differs from the URL above. Links back into DBmarlin use it. |
| Timeout, Max concurrent requests | Limits per request, and across all panels, to protect DBmarlin. |
| SQL text | Show full SQL, truncated SQL, or none, for sites that must not expose statements. |

## Query types

| Query type | Shows |
| --- | --- |
| Activity time series | Database time per instance, optionally grouped, with top N and Other. |
| Activity top N table | Top values of one dimension, with links to DBmarlin. |
| Activity total per instance | Total activity of each instance over the time range, for comparing instances. |
| SQL statistics top N table | Statements ranked by a SQL statistic, with extra columns. |
| Database statistics | Selected statistics of the instance's database type over time. |
| Host metrics | CPU, memory, I/O and disk utilisation of the instance's hosts. |

Alerts, changes and custom events are added through the panel's **Annotations** settings, not as a query type.

## Learn more

- [DBmarlin](https://www.dbmarlin.com)
