/*
外部用户配额管理页面
- 查看所有外部用户
- 调整用户配额
- 设置 VIP 状态
*/

import React, { useEffect, useState } from 'react';
import { 
  Table, Button, Typography, Tag, Modal, Form, InputNumber, 
  Spin, Toast, Popconfirm, Switch, DatePicker, Space, Input
} from '@douyinfe/semi-ui';
import { 
  IconRefresh, IconEdit, IconSearch, IconCrown, 
  IconCheckCircleStroked, IconClose 
} from '@douyinfe/semi-icons';
import { API, showError, showSuccess } from '../../../helpers';

const { Title, Text } = Typography;

export default function SettingsExternalUserManage() {
  const [loading, setLoading] = useState(false);
  const [users, setUsers] = useState([]);
  const [searchText, setSearchText] = useState('');
  
  // 编辑配额弹窗
  const [quotaModalVisible, setQuotaModalVisible] = useState(false);
  const [editingUser, setEditingUser] = useState(null);
  const [newQuota, setNewQuota] = useState(0);
  
  // VIP 弹窗
  const [vipModalVisible, setVipModalVisible] = useState(false);
  const [vipDays, setVipDays] = useState(30);

  // 获取用户列表
  const fetchUsers = async () => {
    setLoading(true);
    try {
      const res = await API.get('/api/external-users/');
      if (res.data.success) {
        setUsers(res.data.data || []);
      } else {
        showError(res.data.message || '获取用户列表失败');
      }
    } catch (error) {
      showError('获取用户列表失败: ' + error.message);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchUsers();
  }, []);

  // 打开配额编辑弹窗
  const openQuotaModal = (user) => {
    setEditingUser(user);
    setNewQuota(user.quotaUsed);
    setQuotaModalVisible(true);
  };

  // 保存配额
  const saveQuota = async () => {
    if (!editingUser) return;
    
    try {
      const res = await API.put(`/api/external-users/${editingUser.id}/quota`, {
        usedCount: newQuota
      });
      if (res.data.success) {
        showSuccess('配额更新成功');
        setQuotaModalVisible(false);
        fetchUsers();
      } else {
        showError(res.data.message);
      }
    } catch (error) {
      showError('更新失败: ' + error.message);
    }
  };

  // 重置配额
  const resetQuota = async (userId) => {
    try {
      const res = await API.put(`/api/external-users/${userId}/quota`, {
        reset: true
      });
      if (res.data.success) {
        showSuccess('配额已重置');
        fetchUsers();
      } else {
        showError(res.data.message);
      }
    } catch (error) {
      showError('重置失败: ' + error.message);
    }
  };

  // 打开 VIP 弹窗
  const openVipModal = (user) => {
    setEditingUser(user);
    setVipDays(30);
    setVipModalVisible(true);
  };

  // 设置 VIP
  const setVip = async (isVip) => {
    if (!editingUser) return;
    
    try {
      const res = await API.put(`/api/external-users/${editingUser.id}/vip`, {
        isVip: isVip,
        vipDays: isVip ? vipDays : 0
      });
      if (res.data.success) {
        showSuccess(isVip ? 'VIP 已开通' : 'VIP 已取消');
        setVipModalVisible(false);
        fetchUsers();
      } else {
        showError(res.data.message);
      }
    } catch (error) {
      showError('操作失败: ' + error.message);
    }
  };

  // 过滤用户
  const filteredUsers = users.filter(user => {
    if (!searchText) return true;
    const search = searchText.toLowerCase();
    return (
      user.email?.toLowerCase().includes(search) ||
      user.username?.toLowerCase().includes(search) ||
      user.id?.toLowerCase().includes(search)
    );
  });

  // 表格列定义
  const columns = [
    {
      title: '用户',
      dataIndex: 'email',
      render: (text, record) => (
        <div>
          <div style={{ fontWeight: 500 }}>{record.username || '未知'}</div>
          <Text type="tertiary" size="small">{text}</Text>
        </div>
      ),
    },
    {
      title: '状态',
      dataIndex: 'isVip',
      width: 100,
      render: (isVip, record) => {
        const now = Date.now() / 1000;
        const isActiveVip = isVip && record.vipExpiresAt > now;
        return isActiveVip ? (
          <Tag color="gold" prefixIcon={<IconCrown />}>VIP</Tag>
        ) : (
          <Tag color="grey">普通</Tag>
        );
      },
    },
    {
      title: '本月配额',
      dataIndex: 'quotaUsed',
      width: 150,
      render: (used, record) => {
        const total = record.quotaTotal;
        const isVip = total === -1; // -1 表示无限（VIP）
        
        if (isVip) {
          return (
            <div>
              <Tag color="gold">{used} / ∞</Tag>
              <Text type="tertiary" size="small" style={{ marginLeft: 4 }}>
                (VIP 无限)
              </Text>
            </div>
          );
        }
        
        const percent = (used / total) * 100;
        const color = percent >= 100 ? 'red' : percent >= 80 ? 'orange' : 'green';
        return (
          <div>
            <Tag color={color}>{used} / {total}</Tag>
            <Text type="tertiary" size="small" style={{ marginLeft: 4 }}>
              ({record.monthKey || '本月'})
            </Text>
          </div>
        );
      },
    },
    {
      title: 'VIP 到期',
      dataIndex: 'vipExpiresAt',
      width: 120,
      render: (timestamp) => {
        if (!timestamp || timestamp === 0) return <Text type="tertiary">-</Text>;
        const date = new Date(timestamp * 1000);
        const isExpired = date < new Date();
        return (
          <Text type={isExpired ? 'danger' : 'success'}>
            {date.toLocaleDateString()}
          </Text>
        );
      },
    },
    {
      title: '操作',
      width: 200,
      render: (_, record) => (
        <Space>
          <Button 
            size="small" 
            icon={<IconEdit />}
            onClick={() => openQuotaModal(record)}
          >
            配额
          </Button>
          <Button 
            size="small" 
            type="warning"
            icon={<IconCrown />}
            onClick={() => openVipModal(record)}
          >
            VIP
          </Button>
          <Popconfirm
            title="确定重置该用户的配额为 0？"
            onConfirm={() => resetQuota(record.id)}
          >
            <Button size="small" type="danger">重置</Button>
          </Popconfirm>
        </Space>
      ),
    },
  ];

  return (
    <Spin spinning={loading}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
        <Title heading={5} style={{ margin: 0 }}>
          外部用户配额管理
        </Title>
        <Space>
          <Input
            prefix={<IconSearch />}
            placeholder="搜索用户..."
            value={searchText}
            onChange={setSearchText}
            style={{ width: 200 }}
          />
          <Button 
            icon={<IconRefresh />} 
            onClick={fetchUsers}
            loading={loading}
          >
            刷新
          </Button>
        </Space>
      </div>

      <Table
        columns={columns}
        dataSource={filteredUsers}
        rowKey="id"
        pagination={{
          pageSize: 20,
          showTotal: true,
        }}
        empty={
          <div style={{ padding: 40, textAlign: 'center' }}>
            <Text type="tertiary">暂无外部用户数据</Text>
            <br />
            <Text type="tertiary" size="small">
              用户在前端登录后会自动创建
            </Text>
          </div>
        }
      />

      {/* 配额编辑弹窗 */}
      <Modal
        title={`编辑配额 - ${editingUser?.username || editingUser?.email}`}
        visible={quotaModalVisible}
        onOk={saveQuota}
        onCancel={() => setQuotaModalVisible(false)}
        okText="保存"
        cancelText="取消"
      >
        <Form>
          <Form.Slot label="当前已用配额">
            <InputNumber
              value={newQuota}
              onChange={setNewQuota}
              min={0}
              max={9999}
              style={{ width: '100%' }}
            />
          </Form.Slot>
          <Form.Slot label="每月总配额">
            <Text>{editingUser?.quotaTotal || 30} 次</Text>
          </Form.Slot>
        </Form>
        <div style={{ marginTop: 12, padding: 8, background: 'var(--semi-color-fill-0)', borderRadius: 4 }}>
          <Text type="secondary" size="small">
            提示：将已用配额设为 0 可重置用户本月配额
          </Text>
        </div>
      </Modal>

      {/* VIP 设置弹窗 */}
      <Modal
        title={`VIP 设置 - ${editingUser?.username || editingUser?.email}`}
        visible={vipModalVisible}
        onCancel={() => setVipModalVisible(false)}
        footer={
          <Space>
            <Button onClick={() => setVipModalVisible(false)}>取消</Button>
            <Button type="danger" onClick={() => setVip(false)}>取消 VIP</Button>
            <Button type="primary" theme="solid" onClick={() => setVip(true)}>
              开通 VIP
            </Button>
          </Space>
        }
      >
        <Form>
          <Form.Slot label="当前状态">
            {editingUser?.isVip ? (
              <Tag color="gold" prefixIcon={<IconCrown />}>VIP 用户</Tag>
            ) : (
              <Tag color="grey">普通用户</Tag>
            )}
          </Form.Slot>
          <Form.Slot label="VIP 时长（天）">
            <InputNumber
              value={vipDays}
              onChange={setVipDays}
              min={1}
              max={3650}
              style={{ width: '100%' }}
              suffix="天"
            />
          </Form.Slot>
        </Form>
        <div style={{ marginTop: 12, padding: 8, background: 'var(--semi-color-fill-0)', borderRadius: 4 }}>
          <Text type="secondary" size="small">
            VIP 用户不受配额限制，可无限使用 API
          </Text>
        </div>
      </Modal>
    </Spin>
  );
}
