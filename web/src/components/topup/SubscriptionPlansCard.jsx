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

import React, { useMemo, useState } from 'react';
import {
  Avatar,
  Badge,
  Button,
  Card,
  Divider,
  Skeleton,
  Space,
  Tag,
  Tooltip,
  Typography,
} from '@douyinfe/semi-ui';
import { API, showError, showSuccess, renderQuota } from '../../helpers';
import { getCurrencyConfig } from '../../helpers/render';
import { Sparkles } from 'lucide-react';
import SubscriptionPurchaseModal from './modals/SubscriptionPurchaseModal';
import {
  formatSubscriptionDuration,
  formatSubscriptionResetPeriod,
} from '../../helpers/subscriptionFormat';

const { Text } = Typography;

// 过滤易支付方式
function getEpayMethods(payMethods = []) {
  return (payMethods || []).filter(
    (m) => m?.type && m.type !== 'stripe' && m.type !== 'creem',
  );
}

// 提交易支付表单
function submitEpayForm({ url, params }) {
  const form = document.createElement('form');
  form.action = url;
  form.method = 'POST';
  const isSafari =
    navigator.userAgent.indexOf('Safari') > -1 &&
    navigator.userAgent.indexOf('Chrome') < 1;
  if (!isSafari) form.target = '_blank';
  Object.keys(params || {}).forEach((key) => {
    const input = document.createElement('input');
    input.type = 'hidden';
    input.name = key;
    input.value = params[key];
    form.appendChild(input);
  });
  document.body.appendChild(form);
  form.submit();
  document.body.removeChild(form);
}

const SubscriptionPlansCard = ({
  t,
  loading = false,
  plans = [],
  payMethods = [],
  enableOnlineTopUp = false,
  enableStripeTopUp = false,
  enableCreemTopUp = false,
  allSubscriptions = [],
  withCard = true,
  title,
  userState,
  reloadUserQuota,
  reloadSubscriptionSelf,
}) => {
  const [open, setOpen] = useState(false);
  const [selectedPlan, setSelectedPlan] = useState(null);
  const [paying, setPaying] = useState(false);
  const [selectedEpayMethod, setSelectedEpayMethod] = useState('');

  const epayMethods = useMemo(() => getEpayMethods(payMethods), [payMethods]);

  const openBuy = (p) => {
    setSelectedPlan(p);
    setSelectedEpayMethod(epayMethods?.[0]?.type || '');
    setOpen(true);
  };

  const closeBuy = () => {
    setOpen(false);
    setSelectedPlan(null);
    setPaying(false);
  };

  const payStripe = async () => {
    if (!selectedPlan?.plan?.stripe_price_id) {
      showError(t('该套餐未配置 Stripe'));
      return;
    }
    setPaying(true);
    try {
      const res = await API.post('/api/subscription/stripe/pay', {
        plan_id: selectedPlan.plan.id,
      });
      if (res.data?.message === 'success') {
        window.open(res.data.data?.pay_link, '_blank');
        showSuccess(t('已打开支付页面'));
        closeBuy();
      } else {
        const errorMsg =
          typeof res.data?.data === 'string'
            ? res.data.data
            : res.data?.message || t('支付失败');
        showError(errorMsg);
      }
    } catch (e) {
      showError(t('支付请求失败'));
    } finally {
      setPaying(false);
    }
  };

  const payCreem = async () => {
    if (!selectedPlan?.plan?.creem_product_id) {
      showError(t('该套餐未配置 Creem'));
      return;
    }
    setPaying(true);
    try {
      const res = await API.post('/api/subscription/creem/pay', {
        plan_id: selectedPlan.plan.id,
      });
      if (res.data?.message === 'success') {
        window.open(res.data.data?.checkout_url, '_blank');
        showSuccess(t('已打开支付页面'));
        closeBuy();
      } else {
        const errorMsg =
          typeof res.data?.data === 'string'
            ? res.data.data
            : res.data?.message || t('支付失败');
        showError(errorMsg);
      }
    } catch (e) {
      showError(t('支付请求失败'));
    } finally {
      setPaying(false);
    }
  };

  const payEpay = async () => {
    if (!selectedEpayMethod) {
      showError(t('请选择支付方式'));
      return;
    }
    setPaying(true);
    try {
      const res = await API.post('/api/subscription/epay/pay', {
        plan_id: selectedPlan.plan.id,
        payment_method: selectedEpayMethod,
      });
      if (res.data?.message === 'success') {
        submitEpayForm({ url: res.data.url, params: res.data.data });
        showSuccess(t('已发起支付'));
        closeBuy();
      } else {
        const errorMsg =
          typeof res.data?.data === 'string'
            ? res.data.data
            : res.data?.message || t('支付失败');
        showError(errorMsg);
      }
    } catch (e) {
      showError(t('支付请求失败'));
    } finally {
      setPaying(false);
    }
  };

  const payWallet = async () => {
    setPaying(true);
    try {
      const res = await API.post('/api/subscription/buy', {
        plan_id: selectedPlan.plan.id,
      });
      if (res.data?.success) {
        showSuccess(t('购买成功！'));
        closeBuy();
        reloadUserQuota?.();
        reloadSubscriptionSelf?.();
      } else {
        showError(res.data?.message || t('购买失败'));
      }
    } catch (e) {
      showError(t('支付请求失败'));
    } finally {
      setPaying(false);
    }
  };

  const planPurchaseCountMap = useMemo(() => {
    const map = new Map();
    (allSubscriptions || []).forEach((sub) => {
      const planId = sub?.subscription?.plan_id;
      if (!planId) return;
      map.set(planId, (map.get(planId) || 0) + 1);
    });
    return map;
  }, [allSubscriptions]);

  const getPlanPurchaseCount = (planId) =>
    planPurchaseCountMap.get(planId) || 0;

  const cardContent = (
    <>
      {loading ? (
        <div className='grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-2 xl:grid-cols-3 gap-5 w-full px-1'>
          {[1, 2, 3].map((i) => (
            <Card
              key={i}
              className='!rounded-xl w-full h-full'
              bodyStyle={{ padding: 16 }}
            >
              <Skeleton.Title active style={{ width: '60%', height: 24, marginBottom: 8 }} />
              <Skeleton.Paragraph active rows={1} style={{ marginBottom: 12 }} />
              <div className='text-center py-4'>
                <Skeleton.Title active style={{ width: '40%', height: 32, margin: '0 auto' }} />
              </div>
              <Skeleton.Paragraph active rows={3} style={{ marginTop: 12 }} />
              <Skeleton.Button active block style={{ marginTop: 16, height: 32 }} />
            </Card>
          ))}
        </div>
      ) : plans.length > 0 ? (
        <div className='grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-2 xl:grid-cols-3 gap-5 w-full px-1'>
          {plans.map((p, index) => {
            const plan = p?.plan;
            const totalAmount = Number(plan?.total_amount || 0);
            const { symbol, rate } = getCurrencyConfig();
            const price = Number(plan?.price_amount || 0);
            const convertedPrice = price * rate;
            const displayPrice = convertedPrice.toFixed(
              Number.isInteger(convertedPrice) ? 0 : 2,
            );
            const isPopular = index === 0 && plans.length > 1;
            const limit = Number(plan?.max_purchase_per_user || 0);
            const limitLabel = limit > 0 ? `${t('限购')} ${limit}` : null;
            const totalLabel =
              totalAmount > 0
                ? `${t('总额度')}: ${renderQuota(totalAmount)}`
                : `${t('总额度')}: ${t('不限')}`;
            const resetLabel =
              formatSubscriptionResetPeriod(plan, t) === t('不重置')
                ? null
                : `${t('额度重置')}: ${formatSubscriptionResetPeriod(plan, t)}`;
            const rateLimitTotal = Number(plan?.request_rate_limit_num || 0);
            const rateLimitSuccess = Number(plan?.request_rate_limit_success || 0);
            const rateLimitDuration = Number(plan?.request_rate_limit_duration || 0);
            const rateLimitLabel =
              rateLimitTotal > 0
                ? `${t('总请求限制')}: ${rateLimitTotal}/${rateLimitDuration}s`
                : null;
            const successLimitLabel =
              rateLimitSuccess > 0
                ? `${t('成功限制')}: ${rateLimitSuccess}/${rateLimitDuration}s`
                : null;

            const planBenefits = [
              {
                label: `${t('有效期')}: ${formatSubscriptionDuration(plan, t)}`,
              },
              resetLabel ? { label: resetLabel } : null,
              totalAmount > 0
                ? {
                    label: totalLabel,
                    tooltip: `${t('原生额度')}：${totalAmount}`,
                  }
                : { label: totalLabel },
              rateLimitLabel ? { label: rateLimitLabel } : null,
              successLimitLabel ? { label: successLimitLabel } : null,
              limitLabel ? { label: limitLabel } : null,
            ].filter(Boolean);

            return (
              <Card
                key={plan?.id}
                className='!rounded-xl transition-all hover:shadow-lg w-full h-full'
                bodyStyle={{ padding: 0 }}
              >
                <div className='p-4 h-full flex flex-col items-center text-center'>
                  {/* 套餐名称 */}
                  <div className='mb-3 w-full'>
                    <Typography.Title
                      heading={5}
                      ellipsis={{ rows: 1, showTooltip: true }}
                      style={{ margin: 0 }}
                    >
                      {plan?.title || t('订阅套餐')}
                    </Typography.Title>
                    {plan?.subtitle && (
                      <Text
                        type='tertiary'
                        size='small'
                        ellipsis={{ rows: 1, showTooltip: true }}
                        style={{ display: 'block' }}
                      >
                        {plan.subtitle}
                      </Text>
                    )}
                  </div>

                  {/* 价格区域 */}
                  <div className='py-2 w-full'>
                    <div className='flex items-baseline justify-center'>
                      <span className='text-xl font-bold text-purple-600'>
                        {symbol}
                      </span>
                      <span className='text-3xl font-bold text-purple-600'>
                        {displayPrice}
                      </span>
                    </div>
                  </div>

                  {/* 套餐权益描述 */}
                  <div className='flex flex-col items-center gap-1 pb-2 w-full'>
                    {planBenefits.map((item) => {
                      const content = (
                        <div className='flex items-center gap-2 text-xs text-gray-500 justify-center'>
                          <Badge dot type='tertiary' />
                          <span>{item.label}</span>
                        </div>
                      );
                      if (!item.tooltip) {
                        return (
                          <div
                            key={item.label}
                            className='w-full flex justify-center'
                          >
                            {content}
                          </div>
                        );
                      }
                      return (
                        <Tooltip key={item.label} content={item.tooltip}>
                          <div className='w-full flex justify-center'>
                            {content}
                          </div>
                        </Tooltip>
                      );
                    })}
                  </div>

                  <div className='mt-auto'>
                    <Divider margin={12} />

                    {/* 购买按钮 */}
                    {(() => {
                      const count = getPlanPurchaseCount(p?.plan?.id);
                      const reached = limit > 0 && count >= limit;
                      const tip = reached
                        ? t('已达到购买上限') + ` (${count}/${limit})`
                        : '';
                      const buttonEl = (
                        <Button
                          theme='outline'
                          type='primary'
                          block
                          disabled={reached}
                          onClick={() => {
                            if (!reached) openBuy(p);
                          }}
                        >
                          {reached ? t('已达上限') : t('立即订阅')}
                        </Button>
                      );
                      return reached ? (
                        <Tooltip content={tip} position='top'>
                          {buttonEl}
                        </Tooltip>
                      ) : (
                        buttonEl
                      );
                    })()}
                  </div>
                </div>
              </Card>
            );
          })}
        </div>
      ) : (
        <div className='text-center text-gray-400 text-sm py-4'>
          {t('暂无可购买套餐')}
        </div>
      )}
    </>
  );

  return (
    <>
      {withCard ? (
        <Card className='!rounded-2xl shadow-sm border-0'>
          {title && (
            <div className='flex items-center justify-between mb-4 px-1'>
              <div className='flex items-center'>
                <Avatar size='small' color='purple' className='mr-3 shadow-md'>
                  <Sparkles size={16} />
                </Avatar>
                <div>
                  <Typography.Text className='text-lg font-medium'>
                    {title}
                  </Typography.Text>
                  <div className='text-xs'>{t('开通订阅，立享模型权益')}</div>
                </div>
              </div>
            </div>
          )}
          {cardContent}
        </Card>
      ) : (
        <div className='space-y-3'>{cardContent}</div>
      )}

      {/* 购买确认弹窗 */}
      <SubscriptionPurchaseModal
        t={t}
        visible={open}
        onCancel={closeBuy}
        selectedPlan={selectedPlan}
        paying={paying}
        selectedEpayMethod={selectedEpayMethod}
        setSelectedEpayMethod={setSelectedEpayMethod}
        epayMethods={epayMethods}
        enableOnlineTopUp={enableOnlineTopUp}
        enableStripeTopUp={enableStripeTopUp}
        enableCreemTopUp={enableCreemTopUp}
        purchaseLimitInfo={
          selectedPlan?.plan?.id
            ? {
                limit: Number(selectedPlan?.plan?.max_purchase_per_user || 0),
                count: getPlanPurchaseCount(selectedPlan?.plan?.id),
              }
            : null
        }
        onPayStripe={payStripe}
        onPayCreem={payCreem}
        onPayEpay={payEpay}
        onPayWallet={payWallet}
        userState={userState}
      />
    </>
  );
};

export default SubscriptionPlansCard;
