import { api } from './api.js';

async function loadFoods() {
    try {
        const foods = await api.getFoods();
        const foodsList = document.getElementById('foods-list');
        foodsList.innerHTML = '';

        if (foods && foods.length > 0) {
            foods.forEach(food => {
                const li = document.createElement('li');
                li.innerHTML = `
                    <div class="food-info">
                        <strong>${food.name}</strong><br>
                        <small>${Math.round(food.calories)} kcal | P: ${Math.round(food.protein)}g | C: ${Math.round(food.carbs)}g | F: ${Math.round(food.fat)}g</small>
                        ${food.nutrients && food.nutrients.length > 0 ?
                        `<br><small><em>${food.nutrients.map(n => `${n.name}: ${n.amount}${n.unit}`).join(', ')}</em></small>`
                        : ''}
                    </div>
                `;
                // Add delete button
                const deleteBtn = document.createElement('button');
                deleteBtn.textContent = 'Delete';
                deleteBtn.onclick = () => deleteFood(food.id);
                li.appendChild(deleteBtn);

                foodsList.appendChild(li);
            });
        } else {
            foodsList.innerHTML = '<li>No foods found.</li>';
        }
    } catch (e) {
        console.error("Failed to load foods:", e);
    }
}

function addNutrientRow() {
    const container = document.getElementById('nutrients-container');
    const row = document.createElement('div');
    row.className = 'nutrient-row';
    row.innerHTML = `
        <input type="text" placeholder="Name (e.g. Vitamin C)" class="nutrient-name" required>
        <input type="number" placeholder="Amount" class="nutrient-amount" step="0.1" required>
        <input type="text" placeholder="Unit (e.g. mg)" class="nutrient-unit" required>
        <button type="button" class="remove-btn" onclick="this.parentElement.remove()">×</button>
    `;
    container.appendChild(row);
}

async function createFood(e) {
    e.preventDefault();
    const form = e.target;

    // Collect nutrients
    const nutrients = [];
    document.querySelectorAll('.nutrient-row').forEach(row => {
        const name = row.querySelector('.nutrient-name').value;
        const amount = parseFloat(row.querySelector('.nutrient-amount').value);
        const unit = row.querySelector('.nutrient-unit').value;
        if (name && !isNaN(amount) && unit) {
            nutrients.push({ name, amount, unit });
        }
    });

    const foodData = {
        name: form.name.value,
        calories: parseFloat(form.calories.value),
        protein: parseFloat(form.protein.value),
        carbs: parseFloat(form.carbs.value),
        fat: parseFloat(form.fat.value),
        nutrients: nutrients
    };

    try {
        await api.createFood(foodData);
        form.reset();
        document.getElementById('nutrients-container').innerHTML = ''; // Clear nutrients
        loadFoods();
    } catch (e) {
        alert("Failed to create food: " + e.message);
    }
}

async function deleteFood(id) {
    if (!confirm("Are you sure you want to delete this food?")) return;
    try {
        await api.deleteFood(id);
        loadFoods();
    } catch (e) {
        alert("Failed to delete food: " + e.message);
    }
}

window.addEventListener('load', () => {
    loadFoods();
    document.getElementById('create-food-form').addEventListener('submit', createFood);
    document.getElementById('add-nutrient-btn').addEventListener('click', addNutrientRow);
});
