import { DataSourceJsonData } from '@grafana/data';
import { DataQuery } from '@grafana/schema';

export type QueryType = 'activity_timeseries' | 'activity_table' | 'activity_summary' | 'db_statistic' | 'host_statistic' | 'events' | 'sql_statistics';

export type HostMetric = 'cpuutilisation' | 'memoryutilisation' | 'ioutilisation' | 'diskutilisation';

export type GroupBy =
  'none' | 'waitevent' | 'sqlhash' | 'session' | 'username' | 'database' | 'client' | 'program' | 'module';

export type EventKind = 'alert' | 'change' | 'custom';

export type Metric = 'aas' | 'waittime' | 'waitcount' | 'avgwaittime';

export interface DBmarlinQuery extends DataQuery {
  queryType: QueryType;
  /** Instance ids, or template variables such as "$instance". */
  instances: string[];
  aggregateInstances: boolean;
  groupBy: GroupBy;
  topN: number;
  includeOther: boolean;
  metric: Metric;
  filters?: Record<string, string[]>;
  /** db_statistic: names from the instance type's catalogue. sql_statistics: extra columns. */
  statistics?: string[];
  /** sql_statistics: the SQL statistic to rank by; duration when empty. */
  orderBy?: string;
  /** host_statistic: host ids; empty follows the selected instances. */
  hosts?: string[];
  hostMetrics?: HostMetric[];
  /** events (annotations): empty means all kinds. */
  eventKinds?: EventKind[];
}

export const DEFAULT_QUERY: Partial<DBmarlinQuery> = {
  queryType: 'activity_timeseries',
  instances: [],
  aggregateInstances: false,
  groupBy: 'none',
  topN: 10,
  includeOther: true,
  metric: 'aas',
};

export type SqlTextMode = 'full' | 'truncated' | 'none';

/**
 * These are options configured for each DataSource instance
 */
export interface MyDataSourceOptions extends DataSourceJsonData {
  url?: string;
  username?: string;
  uiUrl?: string;
  tlsSkipVerify?: boolean;
  /** Seconds, per archiver request. */
  timeout?: number;
  maxConcurrent?: number;
  sqlText?: SqlTextMode;
}

/**
 * Value that is used in the backend, but never sent over HTTP to the frontend
 */
export interface MySecureJsonData {
  password?: string;
  tlsCACert?: string;
}
