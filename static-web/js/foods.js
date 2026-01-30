import { request } from './api.js';
import { injectNavbar } from './components.js';

injectNavbar();

const state = {
    userID: localStorage.getItem('user_id'),
    foods: []
};

if (!state.userID) {
    window.location.href = '/login.html';
}

const els = {
    foodsList: document.getElementById('foodsList'),
    createFoodForm: document.getElementById('createFoodForm'),
    searchInput: document.getElementById('searchInput'),
    toggleFormBtn: document.getElementById('toggleFormBtn'),
    formContainer: document.getElementById('formContainer'),
    addNutrientBtn: document.getElementById('addNutrientBtn'),
    nutrientsContainer: document.getElementById('nutrientsContainer'),
};

// Event Listeners
els.toggleFormBtn.addEventListener('click', () => {
    els.formContainer.classList.toggle('hidden');
    els.toggleFormBtn.textContent = els.formContainer.classList.contains('hidden') ? '+ Create New Food' : 'Cancel';
});

els.addNutrientBtn.addEventListener('click', addNutrientRow);

els.createFoodForm.addEventListener('submit', handleCreateFood);
els.searchInput.addEventListener('input', renderFoods);

// API Calls
async function loadFoods() {
    try {
        const foods = await request('/foods');
        if (foods) {
            state.foods = foods;
            renderFoods();
        }
    } catch (err) {
        console.error('Failed to load foods', err);
    }
}

async function handleCreateFood(e) {
    e.preventDefault();
    const fd = new FormData(els.createFoodForm);
    const data = {
        name: fd.get('name'),
        calories: parseFloat(fd.get('calories')),
        protein: parseFloat(fd.get('protein')),
        carbs: parseFloat(fd.get('carbs')),
        fat: parseFloat(fd.get('fat')),
        type: 'food', // Simple food for now
        measurement_unit: fd.get('measurement_unit'),
        measurement_amount: parseFloat(fd.get('measurement_amount')),
        nutrients: []
    };

    // Collect nutrients
    const nutrientRows = els.nutrientsContainer.querySelectorAll('.nutrient-row');
    nutrientRows.forEach(row => {
        const name = row.querySelector('.nutrient-name').value;
        const amount = parseFloat(row.querySelector('.nutrient-amount').value);
        const unit = row.querySelector('.nutrient-unit').value;
        if (name && !isNaN(amount) && unit) {
            data.nutrients.push({ name, amount, unit });
        }
    });

    try {
        await request('/foods', {
            method: 'POST',
            body: JSON.stringify(data)
        });
        els.createFoodForm.reset();
        els.formContainer.classList.add('hidden');
        els.toggleFormBtn.textContent = '+ Create New Food';
        loadFoods();
    } catch (err) {
        alert('Failed to create food: ' + err.message);
    }
}

async function handleLogFood(foodID, amount, mealTag) {
    // We need to calculate the ratio if they log a different amount, 
    // but the API CreateLog expects 'amount' of the food consumed.
    // The Dashboard display likely depends on the Log entry having the calculated calories?
    // Wait, `api/logs.go` CreateLog takes "Amount" and "FoodID". 
    // It does NOT calculate calories and store them in the Log entry in the DB struct (unless DB triggers do it or query does it).
    // Let's assume the API/DB handles the stat calculation based on FoodID + Amount.
    // Wait, the `GetStats` query joins logs with foods to sum calories.
    // So we just send the amount consumed.

    // For MVP UI, we'll just log 1 serving (MeasurementAmount) or prompt?
    // Let's prompt for amount.

    try {
        await request('/logs', {
            method: 'POST',
            body: JSON.stringify({
                user_id: state.userID,
                food_id: foodID,
                amount: amount,
                meal_tag: mealTag
            })
        });
        alert('Logged!');
    } catch (err) {
        alert('Failed to log');
    }
}

function addNutrientRow() {
    const id = Date.now();
    const div = document.createElement('div');
    div.className = 'nutrient-row';
    div.style.display = 'grid';
    div.style.gridTemplateColumns = '2fr 1fr 1fr auto';
    div.style.gap = '10px';
    div.style.marginBottom = '10px';
    div.id = `nutrient-${id}`;
    div.innerHTML = `
        <input type="text" placeholder="Nutrient (e.g. Vitamin C)" class="nutrient-name" required>
        <input type="number" step="any" placeholder="Amt" class="nutrient-amount" required>
        <input type="text" placeholder="Unit" class="nutrient-unit" required>
        <button type="button" class="remove-nutrient-btn" style="background-color: #d9534f;">X</button>
    `;

    // Add event listener for the remove button
    div.querySelector('.remove-nutrient-btn').addEventListener('click', () => {
        div.remove();
    });

    els.nutrientsContainer.appendChild(div);
}

// Rendering
function renderFoods() {
    const term = els.searchInput.value.toLowerCase();
    const filtered = state.foods.filter(f => f.name.toLowerCase().includes(term));

    els.foodsList.innerHTML = '';

    filtered.forEach(food => {
        const div = document.createElement('div');
        div.className = 'food-item card';
        div.style.padding = '10px';
        div.style.marginBottom = '10px';
        div.innerHTML = `
            <div style="display:flex; justify-content:space-between; align-items:center;">
                <div>
                    <strong>${food.name}</strong>
                    <div style="font-size: 0.8rem; color: #888;">
                        ${food.calories} kcal / ${food.measurement_amount} ${food.measurement_unit} 
                        (P: ${food.protein}g, C: ${food.carbs}g, F: ${food.fat}g)
                    </div>
                </div>
                <button class="log-btn" data-id="${food.id}" data-unit="${food.measurement_unit}" data-amount="${food.measurement_amount}">Log</button>
            </div>
        `;
        els.foodsList.appendChild(div);
    });

    // Add log handlers
    document.querySelectorAll('.log-btn').forEach(btn => {
        btn.addEventListener('click', (e) => {
            const id = e.target.dataset.id;
            const unit = e.target.dataset.unit;
            const defAmount = e.target.dataset.amount;

            // Simple prompt for now
            const amount = prompt(`Amount to log (${unit})?`, defAmount);
            if (amount) {
                const meal = prompt('Meal (Breakfast, Lunch, Dinner, Snack)?', 'Snack');
                if (meal) {
                    handleLogFood(id, parseFloat(amount), meal);
                }
            }
        });
    });
}

loadFoods();
