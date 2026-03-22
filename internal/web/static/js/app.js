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

// Create a line chart
function createLineChart(canvasId, labels, data, label, color) {
    var ctx = document.getElementById(canvasId);
    if (!ctx) return null;

    if (chartInstances[canvasId]) {
        chartInstances[canvasId].destroy();
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
        options: {
            responsive: true,
            maintainAspectRatio: false,
            scales: {
                x: {
                    display: true,
                    grid: { color: '#2a2d3a' },
                    ticks: { color: '#64748b', maxTicksLimit: 8, font: { size: 10 } }
                },
                y: {
                    display: true,
                    grid: { color: '#2a2d3a' },
                    ticks: { color: '#64748b', font: { size: 10 } }
                }
            },
            plugins: {
                legend: { display: false }
            }
        }
    });

    chartInstances[canvasId] = chart;
    return chart;
}

// Fetch and update a chart with history data
function updateChart(chartId, target, metric, window) {
    return fetch('/api/metrics/history?target=' + encodeURIComponent(target) + '&metric=' + encodeURIComponent(metric) + '&window=' + window)
        .then(function(r) { return r.json(); })
        .then(function(data) {
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
                createLineChart(chartId, labels, values, metric, '#3b82f6');
            }
        });
}

// Update all charts for a given target and window
function updateAllCharts(target, window) {
    if (!target || !window) return;

    updateChart('chart-cpu', target, target.startsWith('node:') ? 'node.cpu.percent' : 'vm.cpu.percent', window);
    updateChart('chart-mem', target, target.startsWith('node:') ? 'node.mem.percent' : 'vm.mem.percent', window);
    updateChart('chart-netin', target, 'vm.netin', window);
    updateChart('chart-netout', target, 'vm.netout', window);
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
