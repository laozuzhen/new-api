/*
渠道速率限制监控页面
- 查看所有启用速率限制的渠道
- 显示 RPM/RPD 使用情况和剩余次数
- 支持重置计数
- 支持批量设置速率限制
*/

import React, { useEffect, useState } from 'react';
import { 
  Table, Button, Typography, Tag, Progress, Space, 
  Spin, Popconfirm, Card, Empty, Modal, Form, InputNumber,
  Switch, Select
} from '@douyinfe/semi-ui';
import { 
  IconRefresh, IconDelete, IconAlertCircle, IconSetting
} from '@douyinfe/semi-icons';
import { API, showError, showSuccess } from '../../../helpers';
import { useTranslation } from 'react-i18next';

const { Title, Text } = Typography;

export default function SettingsChannelRateLimit() {
  const { t } = useTranslation();
  const [loading, setLoading] = useState(false);
  const [data, setData] = useState([]);
  
  // 批量设置相关状态
  const [batchModalVisible, setBatchModalVisible] = useState(false);
  const [allChannels, setAllChannels] = useState([]);
  const [selectedChannelIds, setSelectedChannelIds] = useState([]);
  const [batchForm, setBatchForm] = useState({
    rate_limit_enabled: true,
    rate_limit_rpm: 20,
    rate_limit_rpd: 100
  });
  const [batchLoading, setBatchLoading] = useState(false);

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

  // 获取所有渠道列表（用于批量设置）
  const fetchAllChannels = async () => {
    try {
      const res = await API.get('/api/channel/rate_limit/channels');
      if (res.data.success) {
        setAllChannels(res.data.data || []);
      }
    } catch (error) {
      console.error('获取渠道列表失败:', error);
    }
  };

  useEffect(() => {
    fetchData();
    fetchAllChannels();
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

  // 批量设置速率限制
  const handleBatchSubmit = async () => {
    if (selectedChannelIds.length === 0) {
      showError('请选择要设置的渠道');
      return;
    }
    
    setBatchLoading(true);
    try {
      const res = await API.post('/api/channel/rate_limit/batch', {
        ids: selectedChannelIds,
        rate_limit_enabled: batchForm.rate_limit_enabled,
        rate_limit_rpm: batchForm.rate_limit_rpm,
        rate_limit_rpd: batchForm.rate_limit_rpd
      });
      
      if (res.data.success) {
        showSuccess(res.data.message || '批量设置成功');
        setBatchModalVisible(false);
        setSelectedChannelIds([]);
        fetchData();
        fetchAllChannels();
      } else {
        showError(res.data.message || '批量设置失败');
      }
    } catch (error) {
      showError('批量设置失败: ' + error.message);
    } finally {
      setBatchLoading(false);
    }
  };

  const getUsagePercent = (used, limit) => {
    if (limit <= 0) return 0;
    return Math.min(100, Math.round((used / limit) * 100));
  };

  const getProgressColor = (percent) => {
    if (percent >= 90) return 'red';
    if (percent >= 70) return 'orange';
    return 'green';
  };

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

  const totalChannels = data.length;
  const warningChannels = data.filter(d => {
    const rpmPercent = d.rpm_limit > 0 ? (d.rpm_count / d.rpm_limit) * 100 : 0;
    const rpdPercent = d.rpd_limit > 0 ? (d.rpd_count / d.rpd_limit) * 100 : 0;
    return rpmPercent >= 80 || rpdPercent >= 80;
  }).length;

  const channelOptions = allChannels.map(ch => ({
    value: ch.id,
    label: `${ch.name} (ID: ${ch.id})${ch.rate_limit_enabled ? ' ✓' : ''}`,
  }));

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
              icon={<IconSetting />} 
              onClick={() => setBatchModalVisible(true)}
            >
              {t('批量设置')}
            </Button>
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
            pagination={{ pageSize: 20, showTotal: true }}
          />
        </>
      ) : (
        <Empty
          image={<IconAlertCircle size={48} style={{ color: '#bfbfbf' }} />}
          title={t('暂无数据')}
          description={t('没有启用速率限制的渠道，请使用批量设置功能')}
        />
      )}

      <Modal
        title={t('批量设置渠道速率限制')}
        visible={batchModalVisible}
        onCancel={() => setBatchModalVisible(false)}
        onOk={handleBatchSubmit}
        okText={t('确定')}
        cancelText={t('取消')}
        confirmLoading={batchLoading}
        width={600}
      >
        <Form layout="vertical">
          <Form.Slot label={t('选择渠道')}>
            <Select
              multiple
              filter
              style={{ width: '100%' }}
              placeholder={t('请选择要设置的渠道')}
              optionList={channelOptions}
              value={selectedChannelIds}
              onChange={setSelectedChannelIds}
              maxTagCount={5}
            />
            <div style={{ marginTop: 8 }}>
              <Space>
                <Button 
                  size="small" 
                  onClick={() => setSelectedChannelIds(allChannels.map(ch => ch.id))}
                >
                  {t('全选')}
                </Button>
                <Button 
                  size="small" 
                  onClick={() => setSelectedChannelIds(allChannels.filter(ch => !ch.rate_limit_enabled).map(ch => ch.id))}
                >
                  {t('选择未启用的')}
                </Button>
                <Button 
                  size="small" 
                  onClick={() => setSelectedChannelIds([])}
                >
                  {t('清空')}
                </Button>
              </Space>
              <Text type="tertiary" size="small" style={{ marginLeft: 8 }}>
                {t('已选择')} {selectedChannelIds.length} {t('个渠道')}
              </Text>
            </div>
          </Form.Slot>
          
          <Form.Slot label={t('启用速率限制')}>
            <Switch
              checked={batchForm.rate_limit_enabled}
              onChange={(checked) => setBatchForm(prev => ({ ...prev, rate_limit_enabled: checked }))}
            />
          </Form.Slot>
          
          <Form.Slot label={t('每分钟请求限制 (RPM)')}>
            <InputNumber
              style={{ width: '100%' }}
              value={batchForm.rate_limit_rpm}
              onChange={(value) => setBatchForm(prev => ({ ...prev, rate_limit_rpm: value || 0 }))}
              min={0}
              placeholder={t('0 表示不限制')}
            />
            <Text type="tertiary" size="small">
              {t('Gemini 免费 API 通常限制为 15-20 RPM')}
            </Text>
          </Form.Slot>
          
          <Form.Slot label={t('每天请求限制 (RPD)')}>
            <InputNumber
              style={{ width: '100%' }}
              value={batchForm.rate_limit_rpd}
              onChange={(value) => setBatchForm(prev => ({ ...prev, rate_limit_rpd: value || 0 }))}
              min={0}
              placeholder={t('0 表示不限制')}
            />
            <Text type="tertiary" size="small">
              {t('Gemini 免费 API 通常限制为 1500 RPD')}
            </Text>
          </Form.Slot>
        </Form>
      </Modal>
    </Spin>
  );
}
