/*
渠道速率限制监控页面
- 查看所有启用速率限制的渠道
- 显示 RPM/RPD 使用情况和剩余次数
- 支持重置计数
*/

import React, { useEffect, useState } from 'react';
import { 
  Table, Button, Typography, Tag, Progress, Space, 
  Spin, Popconfirm, Card, Empty
} from '@douyinfe/semi-ui';
import { 
  IconRefresh, IconDelete, IconAlertCircle
} from '@douyinfe/semi-icons';
import { API, showError, showSuccess } from '../../../helpers';
import { useTranslation } from 'react-i18next';

const { Title, Text } = Typography;

export default function SettingsChannelRateLimit() {
  const { t } = useTranslation();
  const [loading, setLoading] = useState(false);
  const [data, setData] = useState([]);

  // 获取速率限制数据
  const fetchData = async () => {
    setLoading(true);
    try {
      const res = await API.get('/api/channel/rate_limit');
      if (res.data.success) {
        setData(res.data.data || []);
      } else {
        showError(res.data.message || '获取数据失败');
      }
    } catch (error) {
      showError('获取数据失败: ' + error.message);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchData();
    // 每 30 秒自动刷新
    const interval = setInterval(fetchData, 30000);
    return () => clearInterval(interval);
  }, []);

  // 重置计数
  const resetCount = async (channelId, keyIndex) => {
    try {
      const res = await API.post(`/api/channel/rate_limit/${channelId}/reset`, {
        key_index: keyIndex
      });
      if (res.data.success) {
        showSuccess('计数已重置');
        fetchData();
      } else {
        showError(res.data.message);
      }
    } catch (error) {
      showError('重置失败: ' + error.message);
    }
  };

  // 计算使用百分比
  const getUsagePercent = (used, limit) => {
    if (limit <= 0) return 0;
    return Math.min(100, Math.round((used / limit) * 100));
  };

  // 获取进度条颜色
  const getProgressColor = (percent) => {
    if (percent >= 90) return 'red';
    if (percent >= 70) return 'orange';
    return 'green';
  };

  // 表格列定义
  const columns = [
    {
      title: t('渠道'),
      dataIndex: 'channel_name',
      render: (text, record) => (
        <div>
          <div style={{ fontWeight: 500 }}>{text}</div>
          <Text type="tertiary" size="small">
            ID: {record.channel_id} {record.key_index > 0 ? `| Key #${record.key_index}` : ''}
          </Text>
        </div>
      ),
    },
    {
      title: t('每分钟 (RPM)'),
      dataIndex: 'rpm_count',
      width: 200,
      render: (count, record) => {
        const limit = record.rpm_limit;
        if (limit <= 0) {
          return <Tag color="grey">{t('不限制')}</Tag>;
        }
        const percent = getUsagePercent(count, limit);
        const remaining = record.rpm_remaining;
        return (
          <div>
            <Progress 
              percent={percent} 
              showInfo={false}
              stroke={getProgressColor(percent)}
              style={{ width: 120 }}
            />
            <div style={{ marginTop: 4 }}>
              <Text type={percent >= 90 ? 'danger' : 'secondary'} size="small">
                {count} / {limit} ({t('剩余')}: {remaining})
              </Text>
            </div>
          </div>
        );
      },
    },
    {
      title: t('每天 (RPD)'),
      dataIndex: 'rpd_count',
      width: 200,
      render: (count, record) => {
        const limit = record.rpd_limit;
        if (limit <= 0) {
          return <Tag color="grey">{t('不限制')}</Tag>;
        }
        const percent = getUsagePercent(count, limit);
        const remaining = record.rpd_remaining;
        return (
          <div>
            <Progress 
              percent={percent} 
              showInfo={false}
              stroke={getProgressColor(percent)}
              style={{ width: 120 }}
            />
            <div style={{ marginTop: 4 }}>
              <Text type={percent >= 90 ? 'danger' : 'secondary'} size="small">
                {count} / {limit} ({t('剩余')}: {remaining})
              </Text>
            </div>
          </div>
        );
      },
    },
    {
      title: t('状态'),
      dataIndex: 'enabled',
      width: 80,
      render: (enabled) => (
        enabled ? (
          <Tag color="green">{t('启用')}</Tag>
        ) : (
          <Tag color="grey">{t('禁用')}</Tag>
        )
      ),
    },
    {
      title: t('操作'),
      width: 100,
      render: (_, record) => (
        <Popconfirm
          title={t('确定重置该渠道的计数？')}
          onConfirm={() => resetCount(record.channel_id, record.key_index)}
        >
          <Button size="small" type="danger" icon={<IconDelete />}>
            {t('重置')}
          </Button>
        </Popconfirm>
      ),
    },
  ];

  // 统计信息
  const totalChannels = data.length;
  const warningChannels = data.filter(d => {
    const rpmPercent = d.rpm_limit > 0 ? (d.rpm_count / d.rpm_limit) * 100 : 0;
    const rpdPercent = d.rpd_limit > 0 ? (d.rpd_count / d.rpd_limit) * 100 : 0;
    return rpmPercent >= 80 || rpdPercent >= 80;
  }).length;

  return (
    <Spin spinning={loading}>
      <div style={{ marginBottom: 16 }}>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
          <div>
            <Title heading={5} style={{ margin: 0 }}>
              {t('渠道速率限制监控')}
            </Title>
            <Text type="tertiary" size="small">
              {t('监控启用了速率限制的渠道使用情况')}
            </Text>
          </div>
          <Space>
            {warningChannels > 0 && (
              <Tag color="orange" prefixIcon={<IconAlertCircle />}>
                {warningChannels} {t('个渠道接近限制')}
              </Tag>
            )}
            <Button 
              icon={<IconRefresh />} 
              onClick={fetchData}
              loading={loading}
            >
              {t('刷新')}
            </Button>
          </Space>
        </div>
      </div>

      {data.length > 0 ? (
        <>
          <Card style={{ marginBottom: 16 }}>
            <Space>
              <div>
                <Text type="tertiary">{t('监控渠道数')}</Text>
                <Title heading={4} style={{ margin: 0 }}>{totalChannels}</Title>
              </div>
              <div style={{ marginLeft: 32 }}>
                <Text type="tertiary">{t('接近限制')}</Text>
                <Title heading={4} style={{ margin: 0, color: warningChannels > 0 ? '#f5222d' : 'inherit' }}>
                  {warningChannels}
                </Title>
              </div>
            </Space>
          </Card>

          <Table
            columns={columns}
            dataSource={data}
            rowKey={(record) => `${record.channel_id}-${record.key_index}`}
            pagination={{
              pageSize: 20,
              showTotal: true,
            }}
          />
        </>
      ) : (
        <Empty
          image={<IconAlertCircle size={48} style={{ color: '#bfbfbf' }} />}
          title={t('暂无数据')}
          description={t('没有启用速率限制的渠道，请在渠道设置中启用速率限制')}
        />
      )}
    </Spin>
  );
}
