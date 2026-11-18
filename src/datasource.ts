import { DataSourceInstanceSettings, CoreApp, ScopedVars, SelectableValue } from '@grafana/data';
import { DataSourceWithBackend, getTemplateSrv } from '@grafana/runtime';

import { AnnotationEditor } from './components/AnnotationEditor';
import { VariableSupport } from './variables';
import { DBmarlinQuery, Metric, MyDataSourceOptions, DEFAULT_QUERY } from './types';

export class DataSource extends DataSourceWithBackend<DBmarlinQuery, MyDataSourceOptions> {
  constructor(instanceSettings: DataSourceInstanceSettings<MyDataSourceOptions>) {
    super(instanceSettings);
    this.variables = new VariableSupport(this);
    // DBmarlin alerts, changes and custom events. The backend answers with
    // time/title/text/tags fields, which Grafana maps to annotations itself.
    this.annotations = {
      QueryEditor: AnnotationEditor,
      getDefaultQuery: () => ({ queryType: 'events', instances: [], eventKinds: [] }),
      prepareQuery: (anno) => (anno.target ? { ...anno.target, queryType: 'events' } : undefined),
    };
  }

  getDefaultQuery(_: CoreApp): Partial<DBmarlinQuery> {
    return DEFAULT_QUERY;
  }

  /** Instances the configured DBmarlin user can see, for the query editor. */
  getInstances(): Promise<Array<SelectableValue<string>>> {
    return this.getResource('instances');
  }

  /** Statistics collected for the database type of an instance. */
  getStatistics(instance: string): Promise<Array<SelectableValue<string>>> {
    return this.getResource('statistics', { instance });
  }

  /** SQL statistics collected for the database type of an instance. */
  getSqlStatistics(instance: string): Promise<Array<SelectableValue<string>>> {
    return this.getResource('sqlstatistics', { instance });
  }

  /** Hosts linked to an instance, or all hosts when none is given. */
  getHosts(instance?: string): Promise<Array<SelectableValue<string>>> {
    return this.getResource('hosts', instance ? { instance } : {});
  }

  applyTemplateVariables(query: DBmarlinQuery, scopedVars: ScopedVars): DBmarlinQuery {
    return {
      ...query,
      metric: query.metric ? getTemplateSrv().replace(query.metric, scopedVars) as Metric : query.metric,
      instances: this.expand(query.instances ?? [], scopedVars),
      hosts: this.expand(query.hosts ?? [], scopedVars),
      statistics: this.expand(query.statistics ?? [], scopedVars),
    };
  }

  filterQuery(query: DBmarlinQuery): boolean {
    // host metrics can name hosts directly
    return (
      (query.instances?.length ?? 0) > 0 || (query.queryType === 'host_statistic' && (query.hosts?.length ?? 0) > 0)
    );
  }

  /**
   * Expands template variables, turning a multi-value variable into one entry
   * per value (the backend takes repeated parameters).
   */
  private expand(values: string[], scopedVars: ScopedVars): string[] {
    const templateSrv = getTemplateSrv();
    return values.flatMap((value) => {
      if (!templateSrv.containsTemplate(value)) {
        return [value];
      }
      const json = templateSrv.replace(value, scopedVars, (v: string | string[]) =>
        JSON.stringify(Array.isArray(v) ? v : [v])
      );
      try {
        const parsed = JSON.parse(json);
        return Array.isArray(parsed) ? parsed.map(String) : [json];
      } catch {
        return [json];
      }
    });
  }
}
