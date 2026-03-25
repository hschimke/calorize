// charts.js — Pure Canvas API chart drawing utilities

function getCssVar(name) {
    return getComputedStyle(document.documentElement).getPropertyValue(name).trim();
}

function setupCanvas(canvas) {
    const dpr = window.devicePixelRatio || 1;
    const rect = canvas.getBoundingClientRect();
    if (rect.width === 0 || rect.height === 0) return null;
    canvas.width = rect.width * dpr;
    canvas.height = rect.height * dpr;
    const ctx = canvas.getContext('2d');
    ctx.scale(dpr, dpr);
    return { ctx, w: rect.width, h: rect.height };
}

/**
 * Horizontal stacked bar showing macro calorie contribution.
 * protein × 4 kcal, carbs × 4 kcal, fat × 9 kcal.
 */
export function drawMacroBar(canvas, { protein = 0, carbs = 0, fat = 0 }) {
    const setup = setupCanvas(canvas);
    if (!setup) return;
    const { ctx, w, h } = setup;

    const proteinKcal = protein * 4;
    const carbsKcal = carbs * 4;
    const fatKcal = fat * 9;
    const total = proteinKcal + carbsKcal + fatKcal;

    const labelW = 52;
    const barX = labelW;
    const barW = w - labelW;
    const barH = 20;
    const barY = (h - barH) / 2;

    const colorPrimary = getCssVar('--color-primary');
    const colorCarbs = getCssVar('--color-carbs');
    const colorFat = getCssVar('--color-fat');
    const colorBorder = getCssVar('--color-border');
    const colorMuted = getCssVar('--color-text-muted');

    ctx.font = '11px Inter, -apple-system, sans-serif';
    ctx.fillStyle = colorMuted;
    ctx.textBaseline = 'middle';

    if (total === 0) {
        ctx.fillStyle = colorBorder;
        ctx.fillRect(barX, barY, barW, barH);
        ctx.fillStyle = colorMuted;
        ctx.textAlign = 'center';
        ctx.fillText('No data', barX + barW / 2, barY + barH / 2);
        return;
    }

    const segments = [
        { label: 'P', kcal: proteinKcal, grams: protein, color: colorPrimary },
        { label: 'C', kcal: carbsKcal, grams: carbs, color: colorCarbs },
        { label: 'F', kcal: fatKcal, grams: fat, color: colorFat },
    ];

    const legendItems = [
        { label: `P ${Math.round(protein)}g`, color: colorPrimary },
        { label: `C ${Math.round(carbs)}g`, color: colorCarbs },
        { label: `F ${Math.round(fat)}g`, color: colorFat },
    ];

    // Compact: stack legend vertically if height allows, else inline
    ctx.textAlign = 'left';
    ctx.textBaseline = 'middle';
    const legendLineH = h / 3;
    legendItems.forEach((item, i) => {
        ctx.fillStyle = item.color;
        ctx.fillRect(0, i * legendLineH + legendLineH / 2 - 5, 10, 10);
        ctx.fillStyle = colorMuted;
        ctx.fillText(item.label, 14, i * legendLineH + legendLineH / 2);
    });

    // Draw stacked bar
    let x = barX;
    for (const seg of segments) {
        if (seg.kcal <= 0) continue;
        const segW = (seg.kcal / total) * barW;
        ctx.fillStyle = seg.color;
        ctx.fillRect(x, barY, segW, barH);
        x += segW;
    }
}

/**
 * Horizontal bar chart: 4 rows for breakfast/lunch/dinner/snack.
 * meals = { breakfast, lunch, dinner, snack } in calories.
 */
export function drawMealBars(canvas, meals = {}) {
    const setup = setupCanvas(canvas);
    if (!setup) return;
    const { ctx, w, h } = setup;

    const colorPrimary = getCssVar('--color-primary');
    const colorBorder = getCssVar('--color-border');
    const colorMuted = getCssVar('--color-text-muted');
    const colorText = getCssVar('--color-text');

    const tags = ['breakfast', 'lunch', 'dinner', 'snack'];
    const labels = ['Breakfast', 'Lunch', 'Dinner', 'Snack'];
    const values = tags.map(t => meals[t] || 0);
    const maxVal = Math.max(...values, 1);

    const labelW = 70;
    const valueW = 40;
    const rowH = h / tags.length;
    const barMaxW = w - labelW - valueW - 8;
    const barH = Math.min(rowH * 0.45, 18);

    ctx.font = '12px Inter, -apple-system, sans-serif';

    tags.forEach((tag, i) => {
        const y = i * rowH + rowH / 2;
        const val = values[i];
        const barW = (val / maxVal) * barMaxW;

        // Label
        ctx.fillStyle = colorMuted;
        ctx.textAlign = 'right';
        ctx.textBaseline = 'middle';
        ctx.fillText(labels[i], labelW - 8, y);

        // Bar background
        ctx.fillStyle = colorBorder;
        ctx.fillRect(labelW, y - barH / 2, barMaxW, barH);

        // Bar fill
        if (barW > 0) {
            ctx.fillStyle = colorPrimary;
            ctx.fillRect(labelW, y - barH / 2, barW, barH);
        }

        // Value label
        ctx.fillStyle = colorText;
        ctx.textAlign = 'left';
        ctx.fillText(Math.round(val), labelW + barMaxW + 8, y);
    });
}

/**
 * Vertical bar chart for 7 days (Mon–Sun).
 * days = [{date: 'YYYY-MM-DD', calories: N}, ...]
 */
export function drawDayBars(canvas, days = []) {
    const colors = {
        primary: getCssVar('--color-primary'),
        border: getCssVar('--color-border'),
        muted: getCssVar('--color-text-muted'),
        text: getCssVar('--color-text'),
    };
    const dayLabels = ['Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat', 'Sun'];
    drawVerticalBars(canvas, days, dayLabels, (dateStr, i) => {
        const d = new Date(dateStr + 'T00:00:00');
        const dow = d.getDay(); // 0=Sun
        return dayLabels[dow === 0 ? 6 : dow - 1];
    }, colors);
}

/**
 * Vertical bar chart for weeks in a month.
 * Accepts same per-day array as drawDayBars (28–31 items), aggregates into ISO weeks.
 */
export function drawWeekBars(canvas, days = []) {
    const colors = {
        primary: getCssVar('--color-primary'),
        border: getCssVar('--color-border'),
        muted: getCssVar('--color-text-muted'),
        text: getCssVar('--color-text'),
    };

    // Group days into weeks (chunks of 7 starting from Mon)
    const weeks = [];
    for (let i = 0; i < days.length; i += 7) {
        const chunk = days.slice(i, i + 7);
        const total = chunk.reduce((s, d) => s + (d.calories || 0), 0);
        const startDate = chunk[0]?.date || '';
        weeks.push({ date: startDate, calories: total });
    }

    const weekLabels = weeks.map((w, i) => {
        if (!w.date) return `Wk ${i + 1}`;
        const d = new Date(w.date + 'T00:00:00');
        return `${d.toLocaleString('default', { month: 'short' })} ${d.getDate()}`;
    });

    drawVerticalBars(canvas, weeks, weekLabels, (dateStr, i) => weekLabels[i], colors);
}

function drawVerticalBars(canvas, items, defaultLabels, labelFn, colors) {
    const setup = setupCanvas(canvas);
    if (!setup) return;
    const { ctx, w, h } = setup;

    const paddingTop = 24;
    const paddingBottom = 24;
    const chartH = h - paddingTop - paddingBottom;
    const n = items.length;
    if (n === 0) return;

    const values = items.map(d => d.calories || 0);
    const maxVal = Math.max(...values, 1);

    const barW = Math.floor((w / n) * 0.55);
    const gap = w / n;

    ctx.font = '11px Inter, -apple-system, sans-serif';
    ctx.textBaseline = 'middle';

    items.forEach((item, i) => {
        const val = values[i];
        const barH = Math.max((val / maxVal) * chartH, val > 0 ? 2 : 0);
        const x = gap * i + gap / 2;
        const barTop = paddingTop + chartH - barH;

        // Bar fill
        ctx.fillStyle = val > 0 ? colors.primary : colors.border;
        ctx.fillRect(x - barW / 2, barTop, barW, barH);

        // Value above bar
        ctx.fillStyle = colors.muted;
        ctx.textAlign = 'center';
        ctx.textBaseline = 'bottom';
        if (val > 0) {
            ctx.fillText(Math.round(val), x, barTop - 2);
        }

        // Label below
        ctx.textBaseline = 'top';
        ctx.fillText(labelFn(item.date, i), x, h - paddingBottom + 4);
    });
}
