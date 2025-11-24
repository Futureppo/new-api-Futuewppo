import React, { useState, useEffect } from 'react';
import { Card, Select, Table, Typography, Progress, Tag, Banner, Input, Form, Button } from '@douyinfe/semi-ui';
import { API, showError } from '../../helpers';
import { useTranslation } from 'react-i18next';
import { IconSearch, IconRefresh } from '@douyinfe/semi-icons';

const { Text } = Typography;

const RateLimit = () => {
  const { t } = useTranslation();
  const [channels, setChannels] = useState([]);
  const [selectedChannelId, setSelectedChannelId] = useState(null);
  const [monitorData, setMonitorData] = useState([]);
  const [loading, setLoading] = useState(false);
  const [monitorLoading, setMonitorLoading] = useState(false);
  
  // 筛选状态
  const [filterModel, setFilterModel] = useState('');

  const loadChannels = async () => {
    setLoading(true);
    try {
      const res = await API.get('/api/channel?p=0&size=100');
      const { success, message, data } = res.data;
      if (success) {
        setChannels(data.items);
      } else {
        showError(message);
      }
    } catch (error) {
      showError(error.message);
    } finally {
      setLoading(false);
    }
  };

  const loadMonitorData = async (id) => {
    if (!id) return;
    setMonitorLoading(true);
    try {
      const res = await API.get(`/api/channel/${id}/monitor`);
      const { success, message, data } = res.data;
      if (success) {
        setMonitorData(data);
      } else {
        showError(message);
      }
    } catch (error) {
      showError(error.message);
    } finally {
      setMonitorLoading(false);
    }
  };

  useEffect(() => {
    loadChannels();
  }, []);

  useEffect(() => {
    if (selectedChannelId) {
      loadMonitorData(selectedChannelId);
    } else {
        setMonitorData([]);
    }
  }, [selectedChannelId]);

  const renderLimitColumn = (current, limit) => {
    const percent = limit > 0 ? (current / limit) * 100 : 0;
    const isUnlimited = limit <= 0;
    
    return (
      <div style={{ width: '100%' }}>
        <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 4 }}>
            <Text strong>{current}</Text>
            <Text type="secondary">/ {isUnlimited ? t('无限制') : limit}</Text>
        </div>
        {!isUnlimited && (
            <Progress 
                percent={Math.min(percent, 100)} 
                showInfo={false} 
                size="small" 
                stroke={percent > 90 ? 'var(--semi-color-danger)' : (percent > 70 ? 'var(--semi-color-warning)' : 'var(--semi-color-primary)')} 
            />
        )}
      </div>
    );
  };

  const columns = [
    {
      title: t('模型'),
      dataIndex: 'model_name',
      key: 'model_name',
      render: (text) => <Tag color='blue' size='large' style={{ fontSize: '14px' }}>{text}</Tag>,
      width: 200,
    },
    {
      title: t('RPM (每分钟请求数)'),
      dataIndex: 'current_rpm',
      key: 'rpm',
      render: (text, record) => renderLimitColumn(text, record.limit_rpm),
    },
    {
      title: t('TPM (每分钟Token数)'),
      dataIndex: 'current_tpm',
      key: 'tpm',
      render: (text, record) => renderLimitColumn(text, record.limit_tpm),
    },
    {
      title: t('RPD (每天请求数)'),
      dataIndex: 'current_rpd',
      key: 'rpd',
      render: (text, record) => renderLimitColumn(text, record.limit_rpd),
    },
  ];

  // 前端过滤数据
  const filteredData = monitorData.filter(item => {
    if (!filterModel) return true;
    return item.model_name.toLowerCase().includes(filterModel.toLowerCase());
  });

  return (
    <div className='mt-[60px] px-2'>
      <Banner 
        fullMode={false}
        type="info"
        icon={null}
        closeIcon={null}
        title={t('速率限制管理')}
        description={t('此处为各渠道模型的实时速率限制状态。数据直接来自 Redis 缓存，可能存在轻微延迟。')}
        style={{ marginBottom: 20 }}
      />
      
      <Card>
        <Form layout='horizontal' style={{ marginBottom: 20 }}>
            <Form.Select
                label={t('渠道')}
                style={{ width: 250 }}
                filter
                placeholder={t('搜索并选择渠道...')}
                optionList={channels.map(c => ({ value: c.id, label: `${c.id} - ${c.name} (${c.type === 1 ? 'OpenAI' : 'Other'})` }))}
                onChange={value => setSelectedChannelId(value)}
                loading={loading}
            />
            <Form.Input
                field="model"
                label={t('模型名称')}
                placeholder={t('搜索模型...')}
                style={{ width: 200 }}
                value={filterModel}
                onChange={value => setFilterModel(value)}
                prefix={<IconSearch />}
            />
             <Form.DatePicker
                label={t('时间范围')}
                type="dateTimeRange"
                placeholder={t('选择时间范围')}
                style={{ width: 320 }}
                disabled
                extraText={t('实时监控数据不支持时间筛选')}
            />
            <Button 
                icon={<IconRefresh />} 
                theme='solid' 
                type='primary'
                style={{ marginLeft: 10 }} 
                onClick={() => selectedChannelId && loadMonitorData(selectedChannelId)}
                disabled={!selectedChannelId}
            >
                {t('刷新')}
            </Button>
        </Form>
        
        <Table
            columns={columns}
            dataSource={filteredData}
            loading={monitorLoading}
            pagination={{ pageSize: 20, showSizeChanger: true }}
            emptyText={selectedChannelId ? (filteredData.length === 0 && filterModel ? t('未找到匹配的模型') : t('该渠道暂无模型限制数据或未配置模型')) : t('请先选择一个渠道以查看数据')}
        />
      </Card>
    </div>
  );
};

export default RateLimit;
