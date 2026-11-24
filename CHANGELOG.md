# Changelog

## 1.0.1 (2026-11-24)

### Changes

- The minimum DBmarlin version is now 6.12. The activity endpoints the plugin uses for grouping and top N ship in DBmarlin 6.12, not 6.11 as stated for 1.0.0.

### Requirements

- Grafana 11.0 or later.
- DBmarlin 6.12 or later.

## 1.0.0 (2026-11-18)

Initial release.

### Features

- Activity time series for one or more instances, as average active sessions (AAS), wait time, wait count or average wait time, with optional grouping by wait event, SQL, session, user, database, client, program or module.
- Top N tables for the biggest consumers of database time, with links back to DBmarlin.
- SQL statistics ranked by any statistic DBmarlin collects for the database type.
- Database statistics and host metrics (CPU, memory, I/O, disk).
- Annotations for DBmarlin alerts, changes and custom events.
- Instance dashboard variable.
- Support for Grafana alert rules.
- Two bundled dashboards: DBmarlin - Instance overview and DBmarlin - Fleet overview.

### Requirements

- Grafana 11.0 or later.
- DBmarlin 6.11 or later.
