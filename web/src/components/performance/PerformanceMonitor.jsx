/*
Copyright (C) 2025 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/

import React, { useEffect, useState } from 'react';
import { Card, Col, Row, Typography, Space, Tag, Banner } from '@douyinfe/semi-ui';
import { API, isAdmin } from '../../helpers';
import { useTranslation } from 'react-i18next';

const { Text, Title } = Typography;

const PerformanceMonitor = () => {
  const { t } = useTranslation();
  const [stats, setStats] = useState(null);
  const [loading, setLoading] = useState(true);

  const fetchStats = async () => {
    if (!isAdmin()) return;
    try {
      const res = await API.get('/api/performance/stats');
      const { success, data } = res.data;
      if (success) {
        setStats(data);
      }
    } catch (error) {
      console.error('Failed to fetch performance stats:', error);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchStats();
    const timer = setInterval(fetchStats, 5000); // 5秒刷新一次
    return () => clearInterval(timer);
  }, []);

  if (!isAdmin() || !stats) return null;

  const formatBytes = (bytes) => {
    if (bytes === 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
  };

  return (
    <div style={{ padding: '20px', maxWidth: '1200px', margin: '0 auto' }}>
      <Title heading={3} style={{ marginBottom: '20px' }}>{t('系统实时监控')}</Title>
      <Row gutter={[16, 16]}>
        <Col xs={24} sm={12} md={6}>
          <Card title={t('内存占用')} headerExtraContent={<Tag color='blue'>{t('实时')}</Tag>}>
            <Space vertical align='start'>
              <Text type='secondary'>{t('当前分配')}</Text>
              <Title heading={4}>{formatBytes(stats.memory_stats.alloc)}</Title>
              <Text size='small' type='tertiary'>{t('系统占用')}: {formatBytes(stats.memory_stats.sys)}</Text>
            </Space>
          </Card>
        </Col>
        <Col xs={24} sm={12} md={6}>
          <Card title={t('运行时状态')} headerExtraContent={<Tag color='green'>Go</Tag>}>
            <Space vertical align='start'>
              <Text type='secondary'>{t('Goroutines 数量')}</Text>
              <Title heading={4}>{stats.memory_stats.num_goroutine}</Title>
              <Text size='small' type='tertiary'>{t('累计 GC 次数')}: {stats.memory_stats.num_gc}</Text>
            </Space>
          </Card>
        </Col>
        <Col xs={24} sm={12} md={6}>
          <Card title={t('磁盘缓存')} headerExtraContent={<Tag color='orange'>{t('缓存')}</Tag>}>
            <Space vertical align='start'>
              <Text type='secondary'>{t('缓存文件数')}</Text>
              <Title heading={4}>{stats.disk_cache_info.file_count}</Title>
              <Text size='small' type='tertiary'>{t('总大小')}: {formatBytes(stats.disk_cache_info.total_size)}</Text>
            </Space>
          </Card>
        </Col>
        <Col xs={24} sm={12} md={6}>
          <Card title={t('缓存统计')} headerExtraContent={<Tag color='purple'>{t('命中')}</Tag>}>
            <Space vertical align='start'>
              <Text type='secondary'>{t('总命中次数')}</Text>
              <Title heading={4}>{stats.cache_stats.hits || 0}</Title>
              <Text size='small' type='tertiary'>{t('未命中')}: {stats.cache_stats.misses || 0}</Text>
            </Space>
          </Card>
        </Col>
      </Row>
      {stats.config.monitor_enabled && (
        <Banner
          fullMode={false}
          type="info"
          bordered
          icon={null}
          closeIcon={null}
          style={{ marginTop: 16 }}
          description={`${t('系统健康检查已启用')}: CPU 阈值 ${stats.config.monitor_cpu_threshold}%, 内存阈值 ${stats.config.monitor_memory_threshold}%`}
        />
      )}
    </div>
  );
};

export default PerformanceMonitor;
