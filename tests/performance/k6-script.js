// tests/performance/k6-script.js
import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate } from 'k6/metrics';

// Custom metrics
const errorRate = new Rate('errors');

// Test configuration
export const options = {
  stages: [
    { duration: '2m', target: 50 },  // Ramp up to 50 users
    { duration: '5m', target: 50 },  // Stay at 50 users
    { duration: '2m', target: 100 }, // Ramp up to 100 users
    { duration: '5m', target: 100 }, // Stay at 100 users
    { duration: '2m', target: 0 },   // Ramp down to 0 users
  ],
  thresholds: {
    http_req_duration: ['p(95)<500'], // 95% of requests should be below 500ms
    http_req_failed: ['rate<0.01'],   // Error rate should be less than 1%
    errors: ['rate<0.1'],
  },
};

const API_URL = __ENV.API_URL || 'http://localhost:8080';

// Test data
const testCards = {
  visa: '4242424242424242',
  mastercard: '5555555555554444',
  amex: '378282246310005',
};

const currencies = ['USD', 'EUR', 'GBP', 'JPY'];

function randomCard() {
  const cards = Object.values(testCards);
  return cards[Math.floor(Math.random() * cards.length)];
}

function randomCurrency() {
  return currencies[Math.floor(Math.random() * currencies.length)];
}

function randomAmount() {
  return (Math.random() * 1000 + 10).toFixed(2);
}

// Main test scenario
export default function () {
  const scenario = Math.random();

  if (scenario < 0.4) {
    // 40% - Create payment
    testCreatePayment();
  } else if (scenario < 0.7) {
    // 30% - Currency conversion
    testCurrencyConversion();
  } else if (scenario < 0.9) {
    // 20% - Check payment status
    testGetPayment();
  } else {
    // 10% - List payments
    testListPayments();
  }

  sleep(1);
}

function testCreatePayment() {
  const payload = JSON.stringify({
    amount: parseFloat(randomAmount()),
    currency: randomCurrency(),
    card_number: randomCard(),
    card_exp_month: 12,
    card_exp_year: 2025,
    card_cvc: '123',
    customer_email: `test${Date.now()}@example.com`,
    description: 'Load test payment',
    idempotency_key: `load_test_${Date.now()}_${Math.random()}`,
  });

  const params = {
    headers: {
      'Content-Type': 'application/json',
    },
    tags: { name: 'CreatePayment' },
  };

  const response = http.post(`${API_URL}/api/v1/payments`, payload, params);

  const result = check(response, {
    'create payment status is 200 or 201': (r) => r.status === 200 || r.status === 201,
    'create payment response has id': (r) => {
      try {
        const body = JSON.parse(r.body);
        return body.payment && body.payment.id;
      } catch (e) {
        return false;
      }
    },
    'create payment response time < 1s': (r) => r.timings.duration < 1000,
  });

  errorRate.add(!result);

  if (response.status === 200 || response.status === 201) {
    try {
      const body = JSON.parse(response.body);
      // Store payment ID for subsequent tests
      __VU.paymentId = body.payment.id;
    } catch (e) {
      console.error('Failed to parse payment response:', e);
    }
  }
}

function testGetPayment() {
  if (!__VU.paymentId) {
    // If we don't have a payment ID, create one first
    testCreatePayment();
    return;
  }

  const params = {
    tags: { name: 'GetPayment' },
  };

  const response = http.get(`${API_URL}/api/v1/payments/${__VU.paymentId}`, params);

  const result = check(response, {
    'get payment status is 200': (r) => r.status === 200,
    'get payment has valid data': (r) => {
      try {
        const body = JSON.parse(r.body);
        return body.payment && body.payment.id === __VU.paymentId;
      } catch (e) {
        return false;
      }
    },
    'get payment response time < 200ms': (r) => r.timings.duration < 200,
  });

  errorRate.add(!result);
}

function testListPayments() {
  const params = {
    tags: { name: 'ListPayments' },
  };

  const response = http.get(`${API_URL}/api/v1/payments?limit=10`, params);

  const result = check(response, {
    'list payments status is 200': (r) => r.status === 200,
    'list payments returns array': (r) => {
      try {
        const body = JSON.parse(r.body);
        return Array.isArray(body.payments);
      } catch (e) {
        return false;
      }
    },
    'list payments response time < 300ms': (r) => r.timings.duration < 300,
  });

  errorRate.add(!result);
}

function testCurrencyConversion() {
  const fromCurrency = randomCurrency();
  let toCurrency = randomCurrency();
  
  // Ensure different currencies
  while (toCurrency === fromCurrency) {
    toCurrency = randomCurrency();
  }

  const payload = JSON.stringify({
    amount: parseFloat(randomAmount()),
    from_currency: fromCurrency,
    to_currency: toCurrency,
  });

  const params = {
    headers: {
      'Content-Type': 'application/json',
    },
    tags: { name: 'CurrencyConversion' },
  };

  const response = http.post(`${API_URL}/api/v1/currency/convert`, payload, params);

  const result = check(response, {
    'conversion status is 200': (r) => r.status === 200,
    'conversion has exchange rate': (r) => {
      try {
        const body = JSON.parse(r.body);
        return body.exchange_rate > 0;
      } catch (e) {
        return false;
      }
    },
    'conversion response time < 500ms': (r) => r.timings.duration < 500,
  });

  errorRate.add(!result);
}

// Summary handler
export function handleSummary(data) {
  return {
    'stdout': textSummary(data, { indent: ' ', enableColors: true }),
    'summary.json': JSON.stringify(data),
    'summary.html': htmlReport(data),
  };
}

function textSummary(data, opts) {
  const indent = opts.indent || '';
  let output = `${indent}Test Summary:\n`;
  output += `${indent}  Scenarios: ${data.metrics.iterations.values.count}\n`;
  output += `${indent}  Duration: ${(data.state.testRunDurationMs / 1000).toFixed(2)}s\n`;
  output += `${indent}  HTTP Requests: ${data.metrics.http_reqs.values.count}\n`;
  output += `${indent}  Failed Requests: ${data.metrics.http_req_failed.values.rate.toFixed(2)}%\n`;
  output += `${indent}  Avg Response Time: ${data.metrics.http_req_duration.values.avg.toFixed(2)}ms\n`;
  output += `${indent}  P95 Response Time: ${data.metrics.http_req_duration.values['p(95)'].toFixed(2)}ms\n`;
  return output;
}

function htmlReport(data) {
  return `<!DOCTYPE html>
<html>
<head>
  <title>Load Test Results</title>
  <style>
    body { font-family: Arial, sans-serif; margin: 20px; }
    .metric { margin: 10px 0; }
    .success { color: green; }
    .warning { color: orange; }
    .error { color: red; }
  </style>
</head>
<body>
  <h1>GlobalPay Load Test Results</h1>
  <div class="metric">Total Requests: ${data.metrics.http_reqs.values.count}</div>
  <div class="metric">Average Response Time: ${data.metrics.http_req_duration.values.avg.toFixed(2)}ms</div>
  <div class="metric">P95 Response Time: ${data.metrics.http_req_duration.values['p(95)'].toFixed(2)}ms</div>
  <div class="metric">Failed Requests: ${(data.metrics.http_req_failed.values.rate * 100).toFixed(2)}%</div>
</body>
</html>`;
}