/*
Copyright (C) 2025 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/

import React, { useEffect, useState } from 'react';
import { Descriptions, Tag, Spin, Button, Typography, Banner } from '@douyinfe/semi-ui';
import { IconRefresh, IconCheckCircle, IconCrossCircle, IconInfoCircle } from '@douyinfe/semi-icons';
import { useTranslation } from 'react-i18next';
import { API, showError } from '../../../helpers';

const { Text, Title } = Typography;

export default function SettingsExternalUserAuth() {
  const { t } = useTranslation();
  const [loading, setLoading] = useState(false);
  const [status, setStatus] = useState(null);

  const fetchStatus = async () => {
    setLoading(true);
    try {
      const res = await API.get('/api/external-user-auth/status');
      if (res.data.success) {
        setStatus(res.data.data);
      } else {
        showError(res.data.message || '获取状态失败');
      }
    } catch (error) {
      showError('获取外部用户验证状态失败');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchStatus();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const renderStatusTag = (enabled, trueText = '已配置', falseText = '未配置') => {
    return enabled ? (
      <Tag color="green" prefixIcon={<IconCheckCircle />}>{trueText}</Tag>
    ) : (
      <Tag color="red" prefixIcon={<IconCrossCircle />}>{falseText}</Tag>
    );
  };

  return (
    <Spin spinning={loading}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
        <Title heading={5} style={{ margin: 0 }}>
          {t('外部用户配额验证')}
        </Title>
        <Button 
          icon={<IconRefresh />} 
          onClick={fetchStatus}
          loading={loading}
        >
          {t('刷新')}
        </Button>
      </div>

      {status && (
        <>
          {!status.enabled && (
            <Banner
              type="warning"
              icon={<IconInfoCircle />}
              description={
                <div>
                  <Text strong>配额扣减未生效</Text>
                  <br />
                  <Text type="secondary">{status.disabledReason}</Text>
                </div>
              }
              style={{ marginBottom: 16 }}
            />
          )}

          <Descriptions
            data={[
              {
                key: '系统状态',
                value: status.enabled ? (
                  <Tag color="green" size="large" prefixIcon={<IconCheckCircle />}>
                    已启用 - 配额扣减生效中
                  </Tag>
                ) : (
                  <Tag color="red" size="large" prefixIcon={<IconCrossCircle />}>
                    未启用 - 配额扣减不生效
                  </Tag>
                ),
              },
              {
                key: 'Redis 连接',
                value: renderStatusTag(status.redisConfigured),
              },
              {
                key: 'JWT 密钥',
                value: renderStatusTag(status.jwtConfigured),
              },
              {
                key: '每月配额限制',
                value: <Text>{status.monthlyQuota} 次/月</Text>,
              },
            ]}
            row
            size="medium"
          />

          <div style={{ marginTop: 16, padding: 12, background: 'var(--semi-color-fill-0)', borderRadius: 8 }}>
            <Text type="secondary" size="small">
              <strong>配置说明：</strong>
              <br />
              需要在服务器环境变量中配置以下项目才能启用外部用户配额验证：
              <br />
              • <code>EXTERNAL_USER_REDIS_URL</code> - Upstash Redis REST URL
              <br />
              • <code>EXTERNAL_USER_REDIS_TOKEN</code> - Upstash Redis REST Token
              <br />
              • <code>EXTERNAL_USER_JWT_SECRET</code> - JWT 密钥 (需与前端一致)
              <br />
              • <code>EXTERNAL_USER_MONTHLY_QUOTA</code> - 每月配额 (默认 30)
            </Text>
          </div>
        </>
      )}
    </Spin>
  );
}
