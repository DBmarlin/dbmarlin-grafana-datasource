import React, { ChangeEvent } from 'react';
import { InlineField, Input, SecretInput, SecretTextArea, Select } from '@grafana/ui';
import { DataSourcePluginOptionsEditorProps, SelectableValue } from '@grafana/data';
import { MyDataSourceOptions, MySecureJsonData, SqlTextMode } from '../types';

interface Props extends DataSourcePluginOptionsEditorProps<MyDataSourceOptions, MySecureJsonData> {}

const SQL_TEXT_OPTIONS: Array<SelectableValue<SqlTextMode>> = [
  { label: 'Full', value: 'full' },
  { label: 'Truncated', value: 'truncated' },
  { label: 'None', value: 'none' },
];

export function ConfigEditor(props: Props) {
  const { onOptionsChange, options } = props;
  const { jsonData, secureJsonFields, secureJsonData } = options;

  const setJsonData = (patch: Partial<MyDataSourceOptions>) =>
    onOptionsChange({ ...options, jsonData: { ...jsonData, ...patch } });

  const onText = (key: 'url' | 'username' | 'uiUrl') => (event: ChangeEvent<HTMLInputElement>) =>
    setJsonData({ [key]: event.target.value });

  const onNumber = (key: 'timeout' | 'maxConcurrent') => (event: ChangeEvent<HTMLInputElement>) =>
    setJsonData({ [key]: event.target.value === '' ? undefined : Number(event.target.value) });

  // Secure fields are only sent to the backend, never read back.
  const onSecret = (key: keyof MySecureJsonData) => (event: ChangeEvent<HTMLInputElement | HTMLTextAreaElement>) =>
    onOptionsChange({ ...options, secureJsonData: { ...secureJsonData, [key]: event.target.value } });

  const onResetSecret = (key: keyof MySecureJsonData) => () =>
    onOptionsChange({
      ...options,
      secureJsonFields: { ...secureJsonFields, [key]: false },
      secureJsonData: { ...secureJsonData, [key]: '' },
    });

  return (
    <>
      <InlineField
        label="URL"
        labelWidth={20}
        required
        tooltip="DBmarlin base URL, for example https://dbmarlin.example.com:9090. /archiver/rest is added."
      >
        <Input
          id="config-editor-url"
          value={jsonData.url ?? ''}
          onChange={onText('url')}
          placeholder="https://dbmarlin.example.com:9090"
          width={50}
        />
      </InlineField>
      <InlineField
        label="Username"
        labelWidth={20}
        tooltip="DBmarlin user. Use a dedicated, least-privilege user: everyone who can query this data source sees what that user sees. Leave empty if DBmarlin has no authentication."
      >
        <Input id="config-editor-username" value={jsonData.username ?? ''} onChange={onText('username')} width={30} />
      </InlineField>
      <InlineField label="Password" labelWidth={20}>
        <SecretInput
          id="config-editor-password"
          isConfigured={Boolean(secureJsonFields?.password)}
          value={secureJsonData?.password ?? ''}
          width={30}
          onReset={onResetSecret('password')}
          onChange={onSecret('password')}
        />
      </InlineField>
      <InlineField
        label="TLS CA certificate"
        labelWidth={20}
        tooltip="PEM certificate that signed the DBmarlin certificate, or the certificate itself if it is self-signed."
      >
        <SecretTextArea
          id="config-editor-tls-ca-cert"
          isConfigured={Boolean(secureJsonFields?.tlsCACert)}
          value={secureJsonData?.tlsCACert ?? ''}
          rows={4}
          cols={50}
          placeholder="-----BEGIN CERTIFICATE-----"
          onReset={onResetSecret('tlsCACert')}
          onChange={onSecret('tlsCACert')}
        />
      </InlineField>
      <InlineField
        label="UI URL"
        labelWidth={20}
        tooltip="Base for links back into DBmarlin, if different from the URL above."
      >
        <Input
          id="config-editor-ui-url"
          value={jsonData.uiUrl ?? ''}
          onChange={onText('uiUrl')}
          placeholder={jsonData.url || 'Same as URL'}
          width={50}
        />
      </InlineField>
      <InlineField
        label="Timeout (s)"
        labelWidth={20}
        tooltip="Per DBmarlin request. Keep under nginx's proxy_read_timeout."
      >
        <Input
          id="config-editor-timeout"
          type="number"
          min={1}
          value={jsonData.timeout ?? ''}
          onChange={onNumber('timeout')}
          placeholder="60"
          width={10}
        />
      </InlineField>
      <InlineField label="Max concurrent requests" labelWidth={20} tooltip="Per data source, across all panels.">
        <Input
          id="config-editor-max-concurrent"
          type="number"
          min={1}
          value={jsonData.maxConcurrent ?? ''}
          onChange={onNumber('maxConcurrent')}
          placeholder="8"
          width={10}
        />
      </InlineField>
      <InlineField
        label="SQL text"
        labelWidth={20}
        tooltip="How much SQL text Grafana may show, for sites that must not expose it."
      >
        {/* eslint-disable-next-line @typescript-eslint/no-deprecated -- Combobox needs Grafana 11.3+; we support 11.0 */}
        <Select
          inputId="config-editor-sql-text"
          options={SQL_TEXT_OPTIONS}
          value={jsonData.sqlText ?? 'full'}
          onChange={(option) => setJsonData({ sqlText: option.value })}
          width={20}
        />
      </InlineField>
    </>
  );
}
