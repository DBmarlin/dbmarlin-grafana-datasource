import fs from 'fs';
import path from 'path';
import plugin from '../plugin.json';

const QUERY_TYPES = ['activity_timeseries', 'activity_table', 'activity_summary', 'db_statistic', 'host_statistic', 'events'];
const GROUP_BY = ['none', 'waitevent', 'sqlhash', 'session', 'username', 'database', 'client', 'program', 'module'];

const files = plugin.includes.filter((i) => i.type === 'dashboard').map((i) => i.path);
const load = (file: string) => JSON.parse(fs.readFileSync(path.join(__dirname, '..', file), 'utf8'));

describe('bundled dashboards', () => {
  it('are registered in plugin.json and exist', () => {
    expect(files.length).toBeGreaterThan(0);
    for (const file of files) {
      expect(fs.existsSync(path.join(__dirname, '..', file))).toBe(true);
    }
  });

  describe.each(files)('%s', (file) => {
    const dashboard = load(file);
    const queries = [
      ...dashboard.panels.flatMap((p: any) => p.targets),
      ...dashboard.annotations.list.map((a: any) => a.target),
    ];

    it('has a unique uid, title and panel ids', () => {
      expect(dashboard.uid).toMatch(/^dbmarlin-/);
      expect(dashboard.title).toMatch(/^DBmarlin - /);
      const ids = dashboard.panels.map((p: any) => p.id);
      expect(new Set(ids).size).toBe(ids.length);
    });

    it('picks its data source with the ds variable, never a fixed uid', () => {
      const sources = [
        ...dashboard.panels.flatMap((p: any) => [p.datasource, ...p.targets.map((t: any) => t.datasource)]),
        ...dashboard.annotations.list.map((a: any) => a.datasource),
        dashboard.templating.list.find((v: any) => v.name === 'instance').datasource,
      ];
      for (const s of sources) {
        expect(s).toEqual({ type: plugin.id, uid: '${ds}' });
      }
    });

    it('only uses query types and groupings the plugin supports, for the $instance variable', () => {
      for (const q of queries) {
        expect(QUERY_TYPES).toContain(q.queryType);
        expect(GROUP_BY).toContain(q.groupBy ?? 'none');
        expect(q.instances).toEqual(['$instance']);
        expect(['aas', 'waittime', 'waitcount', 'avgwaittime', '$metric', undefined]).toContain(q.metric);
      }
    });

    it('only links to bundled dashboards', () => {
      const uids = files.map((f) => load(f).uid);
      const urls: string[] = [
        ...dashboard.links.map((l: any) => l.url),
        ...dashboard.panels.flatMap((p: any) => [
          ...(p.fieldConfig.defaults.links ?? []).map((l: any) => l.url),
          ...p.fieldConfig.overrides.flatMap((o: any) =>
            o.properties.filter((x: any) => x.id === 'links').flatMap((x: any) => x.value.map((l: any) => l.url))
          ),
        ]),
      ];
      for (const url of urls) {
        expect(uids.some((uid) => url.startsWith(`/d/${uid}`))).toBe(true);
      }
    });

    it('hides the helper columns of its tables', () => {
      for (const panel of dashboard.panels.filter((p: any) => p.type === 'table')) {
        const hidden = panel.fieldConfig.overrides.find((o: any) => o.matcher.id === 'byRegexp');
        expect(hidden.matcher.options).toBe('^(link|instance_id)$');
      }
    });
  });
});
