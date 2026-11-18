import { test as base, expect } from '@grafana/plugin-e2e';
import type { PanelEditPage } from '@grafana/plugin-e2e';
import type { Page } from '@playwright/test';
import type { DBmarlinQuery } from '../src/types';

// Test the real Grafana editor independently of DBmarlin's licence, catalogue
// and collected data. These responses are editor fixtures, not backend tests.
const test = base.extend({
  panelEditPage: async ({ dashboardPage, readProvisionedDataSource, page }, use) => {
    await dashboardPage.mockResourceResponse('instances', [
      { label: 'test-postgres', value: '3' },
      { label: 'test-mysql', value: '4' },
    ]);
    await dashboardPage.mockResourceResponse('statistics?*', [{ label: 'blks_hit', value: 'blks_hit' }]);
    await dashboardPage.mockResourceResponse('sqlstatistics?*', [
      { label: 'duration', value: 'duration' },
      { label: 'executions', value: 'executions' },
    ]);
    await dashboardPage.mockResourceResponse('hosts?*', [{ label: 'test-linux', value: '7' }]);
    await dashboardPage.mockQueryDataResponse({ results: { A: { status: 200, frames: [] } } });
    const panelEditPage = await dashboardPage.addPanel();
    const ds = await readProvisionedDataSource({ fileName: 'datasources.yml' });
    const picker = page.getByRole('combobox', { name: 'Select a data source', exact: true });
    await picker.fill(ds.name);
    await picker.press('ArrowDown');
    await picker.press('ArrowUp');
    await picker.press('Enter');
    await panelEditPage.setVisualization('Time series');
    await use(panelEditPage);
  },
});

async function select(page: Page, id: string, label: string) {
  await page.locator(`#query-editor-${id}`).click();
  await page.getByRole('option', { name: label, exact: true }).click();
}

// An earlier automatic request must not satisfy an assertion about a change.
async function expectQuery(panel: PanelEditPage, expected: Partial<DBmarlinQuery>, change: () => Promise<unknown>) {
  const request = panel.waitForQueryDataRequest((req) =>
    req
      .postDataJSON()
      .queries.some(
        (query: DBmarlinQuery) =>
          query.refId === 'A' &&
          Object.entries(expected).every(
            ([key, value]) => JSON.stringify(query[key as keyof DBmarlinQuery]) === JSON.stringify(value)
          )
      )
  );
  await change();
  expect((await request).postDataJSON().queries.find((q: DBmarlinQuery) => q.refId === 'A')).toMatchObject(expected);
}

test('renders the default activity time series controls', async ({ panelEditPage, page }) => {
  const editor = panelEditPage.getQueryEditorRow('A');
  await expect(editor).toContainText('Activity time series');
  await expect(editor).toContainText('AAS (average active sessions)');
  await expect(editor.locator('#query-editor-instances')).toBeVisible();
  await expect(editor.locator('#query-editor-groupby')).toBeVisible();
  await expect(editor.locator('#query-editor-aggregate')).not.toBeChecked();
  await expect(editor.locator('#query-editor-topn')).toHaveCount(0);
  await expect(editor.locator('#query-editor-other')).toHaveCount(0);
  await expectQuery(
    panelEditPage,
    {
      queryType: 'activity_timeseries',
      instances: ['3'],
      metric: 'aas',
      groupBy: 'none',
    },
    () => select(page, 'instances', 'test-postgres')
  );
});

test('runs updated queries for metrics, multiple instances and combining instances', async ({
  panelEditPage,
  page,
}) => {
  await select(page, 'instances', 'test-postgres');
  await expectQuery(panelEditPage, { instances: ['3', '4'] }, () => select(page, 'instances', 'test-mysql'));
  await expectQuery(panelEditPage, { metric: 'waittime' }, () => select(page, 'metric', 'Wait time (s)'));
  await expectQuery(panelEditPage, { aggregateInstances: true }, () =>
    page.locator('#query-editor-aggregate + label').click()
  );
});

test('grouping reveals top N and Other and sends their values', async ({ panelEditPage, page }) => {
  await select(page, 'instances', 'test-postgres');
  await expectQuery(panelEditPage, { groupBy: 'waitevent' }, () => select(page, 'groupby', 'Wait event'));
  await expect(page.locator('#query-editor-topn')).toHaveValue('10');
  await expect(page.locator('#query-editor-other')).toBeChecked();
  await expectQuery(panelEditPage, { groupBy: 'waitevent', topN: 5 }, () =>
    page.locator('#query-editor-topn').fill('5')
  );
  await expectQuery(panelEditPage, { includeOther: false }, () => page.locator('#query-editor-other + label').click());
  await select(page, 'groupby', 'None');
  await expect(page.locator('#query-editor-topn')).toHaveCount(0);
  await expect(page.locator('#query-editor-other')).toHaveCount(0);
});

test('activity tables default to a dimension and hide time series controls', async ({ panelEditPage, page }) => {
  await select(page, 'instances', 'test-postgres');
  await expectQuery(panelEditPage, { queryType: 'activity_table', groupBy: 'waitevent' }, () =>
    select(page, 'type', 'Activity top N table')
  );
  await expect(panelEditPage.getQueryEditorRow('A')).toContainText('List');
  for (const id of ['metric', 'aggregate', 'other']) {
    await expect(page.locator(`#query-editor-${id}`)).toHaveCount(0);
  }
  await expect(page.locator('#query-editor-topn')).toBeVisible();
  await expectQuery(panelEditPage, { groupBy: 'sqlhash' }, () => select(page, 'groupby', 'SQL'));
  await page.locator('#query-editor-groupby').click();
  await expect(page.getByRole('option', { name: 'None', exact: true })).toHaveCount(0);
});

test('database statistics use the selected instance catalogue', async ({ panelEditPage, page }) => {
  await select(page, 'type', 'Database statistics');
  await expect(panelEditPage.getQueryEditorRow('A')).toContainText('Select an instance first');
  const catalogue = page.waitForRequest(
    (req) => req.url().includes('/resources/statistics?') && new URL(req.url()).searchParams.get('instance') === '3'
  );
  await select(page, 'instances', 'test-postgres');
  await catalogue;
  await expectQuery(panelEditPage, { queryType: 'db_statistic', instances: ['3'], statistics: ['blks_hit'] }, () =>
    select(page, 'statistics', 'blks_hit')
  );
  await expect(page.locator('#query-editor-groupby')).toHaveCount(0);
  await expect(page.locator('#query-editor-metric')).toHaveCount(0);
});

test('SQL statistics send ranking, top N and extra columns', async ({ panelEditPage, page }) => {
  await select(page, 'instances', 'test-postgres');
  await select(page, 'type', 'SQL statistics top N table');
  await expectQuery(panelEditPage, { queryType: 'sql_statistics', orderBy: 'executions' }, () =>
    select(page, 'orderby', 'executions')
  );
  await expectQuery(panelEditPage, { topN: 7 }, () => page.locator('#query-editor-topn').fill('7'));
  await expectQuery(panelEditPage, { statistics: ['executions'] }, () => select(page, 'sqlcolumns', 'executions'));
  await expect(page.locator('#query-editor-groupby')).toHaveCount(0);
  await expect(page.locator('#query-editor-metric')).toHaveCount(0);
});

test('host metrics send selected hosts and metrics', async ({ panelEditPage, page }) => {
  await select(page, 'instances', 'test-postgres');
  await select(page, 'type', 'Host metrics');
  await expectQuery(panelEditPage, { queryType: 'host_statistic', hosts: ['7'] }, () =>
    select(page, 'hosts', 'test-linux')
  );
  await expectQuery(panelEditPage, { hostMetrics: ['cpuutilisation'] }, () =>
    select(page, 'hostmetrics', 'CPU utilisation')
  );
  await expect(page.locator('#query-editor-groupby')).toHaveCount(0);
  await expect(page.locator('#query-editor-metric')).toHaveCount(0);
});
