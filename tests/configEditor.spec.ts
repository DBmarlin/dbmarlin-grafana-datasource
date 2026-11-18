import { test, expect } from '@grafana/plugin-e2e';

// The success case needs a running DBmarlin: DBMARLIN_URL is the address
// Grafana uses (./dev up prints it), DBMARLIN_USER/DBMARLIN_PASSWORD are optional.
const { DBMARLIN_URL, DBMARLIN_USER, DBMARLIN_PASSWORD } = process.env;

test('smoke: should render config editor', async ({ createDataSourceConfigPage, readProvisionedDataSource, page }) => {
  const ds = await readProvisionedDataSource({ fileName: 'datasources.yml' });
  await createDataSourceConfigPage({ type: ds.type });
  await expect(page.locator('#config-editor-url')).toBeVisible();
});

test('"Save & test" should be successful when configuration is valid', async ({
  createDataSourceConfigPage,
  readProvisionedDataSource,
  page,
}) => {
  test.skip(!DBMARLIN_URL, 'DBMARLIN_URL is not set');
  const ds = await readProvisionedDataSource({ fileName: 'datasources.yml' });
  const configPage = await createDataSourceConfigPage({ type: ds.type });
  await page.locator('#config-editor-url').fill(DBMARLIN_URL ?? '');
  if (DBMARLIN_USER) {
    await page.locator('#config-editor-username').fill(DBMARLIN_USER);
    await page.locator('#config-editor-password').fill(DBMARLIN_PASSWORD ?? '');
  }
  await expect(configPage.saveAndTest()).toBeOK();
});

test('"Save & test" should fail when the URL is missing', async ({
  createDataSourceConfigPage,
  readProvisionedDataSource,
}) => {
  const ds = await readProvisionedDataSource({ fileName: 'datasources.yml' });
  const configPage = await createDataSourceConfigPage({ type: ds.type });
  await expect(configPage.saveAndTest()).not.toBeOK();
  await expect(configPage).toHaveAlert('error', { hasText: 'Enter the DBmarlin URL' });
});
