// SSE connection with auto-reconnect
function connectSSE() {
    const es = new EventSource('/events');

    es.onmessage = function(e) {
        try {
            const data = JSON.parse(e.data);

            // Update badges by data-target and data-check attributes
            document.querySelectorAll('[data-target="' + data.target + '"][data-check="' + data.check + '"]').forEach(function(el) {
                el.className = 'badge badge-' + data.status;
                el.textContent = data.status;
            });

            // Update last-updated timestamp
            const el = document.getElementById('last-updated');
            if (el) el.textContent = new Date().toLocaleTimeString();
        } catch (err) {
            console.error('SSE parse error:', err);
        }
    };

    es.onerror = function() {
        es.close();
        setTimeout(connectSSE, 5000);
    };
}

// Chart instances registry for updates
var chartInstances = {};

// Get Y-axis formatter based on metric name
function getYFormatter(metric) {
    if (!metric) return undefined;

    if (metric.includes('percent')) {
        return function(value) { return value.toFixed(0) + '%'; };
    }
    if (metric.includes('mem') || metric.includes('disk')) {
        return function(value) { return humanBytes(value); };
    }
    if (metric.includes('net')) {
        return function(value) { return humanBytes(value) + '/s'; };
    }
    if (metric.includes('load')) {
        return function(value) { return value.toFixed(1); };
    }
    return undefined;
}

// Get threshold annotation for metric
function getThreshold(metric) {
    if (metric && metric.includes('cpu.percent')) {
        return {
            type: 'line',
            yMin: 80,
            yMax: 80,
            borderColor: '#ef444466',
            borderWidth: 1,
            borderDash: [5, 5],
            label: {
                display: true,
                content: '80%',
                position: 'end',
                color: '#ef4444',
                font: { size: 9 }
            }
        };
    }
    return null;
}

// Create a doughnut gauge (for CPU/RAM)
function createGauge(canvasId, value, label, color) {
    var ctx = document.getElementById(canvasId);
    if (!ctx) return null;

    if (chartInstances[canvasId]) {
        chartInstances[canvasId].destroy();
    }

    var chart = new Chart(ctx, {
        type: 'doughnut',
        data: {
            datasets: [{
                data: [value, 100 - value],
                backgroundColor: [color, '#2a2d3a'],
                borderWidth: 0
            }]
        },
        options: {
            cutout: '75%',
            responsive: true,
            maintainAspectRatio: true,
            plugins: {
                legend: { display: false },
                tooltip: { enabled: false }
            }
        },
        plugins: [{
            id: 'centerText',
            afterDraw: function(chart) {
                var width = chart.width;
                var height = chart.height;
                var ctx = chart.ctx;
                ctx.restore();
                var fontSize = (height / 100).toFixed(2);
                ctx.font = '600 ' + fontSize + 'em JetBrains Mono, monospace';
                ctx.textBaseline = 'middle';
                ctx.fillStyle = '#e2e8f0';
                var text = value.toFixed(0) + '%';
                var textX = Math.round((width - ctx.measureText(text).width) / 2);
                var textY = height / 2;
                ctx.fillText(text, textX, textY);
                ctx.save();
            }
        }]
    });

    chartInstances[canvasId] = chart;
    return chart;
}

// Create a line chart with optional metric-based formatting
function createLineChart(canvasId, labels, data, label, color, metric) {
    var ctx = document.getElementById(canvasId);
    if (!ctx) return null;

    if (chartInstances[canvasId]) {
        chartInstances[canvasId].destroy();
    }

    var yFormatter = getYFormatter(metric);
    var threshold = getThreshold(metric);

    var options = {
        responsive: true,
        maintainAspectRatio: false,
        layout: {
            padding: { top: 5, right: 10, bottom: 5, left: 5 }
        },
        scales: {
            x: {
                display: true,
                grid: { color: '#2a2d3a' },
                ticks: { color: '#64748b', maxTicksLimit: 6, font: { size: 9 } }
            },
            y: {
                display: true,
                grid: { color: '#2a2d3a' },
                ticks: { color: '#64748b', font: { size: 9 }, maxTicksLimit: 5 }
            }
        },
        plugins: {
            legend: { display: false },
            tooltip: {
                callbacks: {
                    label: function(context) {
                        var val = context.parsed.y;
                        if (yFormatter) return yFormatter(val);
                        return val.toFixed(2);
                    }
                }
            }
        }
    };

    if (yFormatter) {
        options.scales.y.ticks.callback = yFormatter;
    }

    if (threshold) {
        options.plugins.annotation = { annotations: { threshold: threshold } };
    }

    var chart = new Chart(ctx, {
        type: 'line',
        data: {
            labels: labels,
            datasets: [{
                label: label,
                data: data,
                borderColor: color || '#3b82f6',
                backgroundColor: (color || '#3b82f6') + '20',
                fill: true,
                tension: 0.3,
                pointRadius: 0,
                borderWidth: 2
            }]
        },
        options: options
    });

    chartInstances[canvasId] = chart;
    return chart;
}

// Fetch and update a chart with window
function updateChart(chartId, target, metric, window) {
    return fetch('/api/metrics/history?target=' + encodeURIComponent(target) + '&metric=' + encodeURIComponent(metric) + '&window=' + window)
        .then(function(r) { return r.json(); })
        .then(function(data) {
            updateChartWithData(chartId, data, metric);
        });
}

// Fetch and update a chart with custom date range
function updateChartRange(chartId, target, metric, from, to) {
    return fetch('/api/metrics/history?target=' + encodeURIComponent(target) + '&metric=' + encodeURIComponent(metric) + '&from=' + from + '&to=' + to)
        .then(function(r) { return r.json(); })
        .then(function(data) {
            updateChartWithData(chartId, data, metric);
        });
}

// Update chart with data array
function updateChartWithData(chartId, data, metric) {
    var labels = data.map(function(p) {
        var d = new Date(p.timestamp);
        return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
    });
    var values = data.map(function(p) { return p.value; });

    if (chartInstances[chartId]) {
        chartInstances[chartId].data.labels = labels;
        chartInstances[chartId].data.datasets[0].data = values;
        chartInstances[chartId].update();
    } else {
        createLineChart(chartId, labels, values, metric, '#3b82f6', metric);
    }
}

// Format bytes to human readable
function humanBytes(bytes) {
    if (bytes === 0) return '0 B';
    var k = 1024;
    var sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
    var i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
}

// Format relative time
function relativeTime(isoString) {
    var date = new Date(isoString);
    var now = new Date();
    var seconds = Math.floor((now - date) / 1000);

    if (seconds < 60) return seconds + 's ago';
    if (seconds < 3600) return Math.floor(seconds / 60) + 'm ago';
    if (seconds < 86400) return Math.floor(seconds / 3600) + 'h ago';
    return Math.floor(seconds / 86400) + 'd ago';
}

// Format uptime seconds to human readable
function formatUptime(seconds) {
    if (seconds <= 0) return '-';
    var d = Math.floor(seconds / 86400);
    var h = Math.floor((seconds % 86400) / 3600);
    var m = Math.floor((seconds % 3600) / 60);
    var parts = [];
    if (d > 0) parts.push(d + 'd');
    if (h > 0) parts.push(h + 'h');
    if (m > 0) parts.push(m + 'm');
    return parts.join(' ') || '< 1m';
}
