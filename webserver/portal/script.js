const recordForm = document.getElementById('recordForm');
const dataTable = document.getElementById('dataTable').getElementsByTagName('tbody')[0];
const callsTable = document.getElementById('callsTable').getElementsByTagName('tbody')[0];
const deleteSelected = document.getElementById('deleteSelected');
const refresh = document.getElementById('refresh');
const submitButton = recordForm.querySelector('button[type="submit"]');
const editData = document.getElementById('editData');
const saveData = document.getElementById('saveData');
const pcscfSocket = document.getElementById('pcscfSocket');
const imsDomain = document.getElementById('imsDomain');
const ringingSound = document.getElementById('ringingSound');
const ws = new WebSocket(`ws://${location.host}/ws`);

const btnRefreshCalls = document.getElementById('btnRefreshCalls');
const btnClearCalls = document.getElementById('btnClearCalls');

const animationProperty1 = 'flashButton 1s infinite'
const animationProperty = 'flash 0.5s infinite alternate'


document.addEventListener('DOMContentLoaded', function () {
    loadData()
    editData.textContent = editData.dataset.actionEdit
    pcscfSocket.disabled = true;
    imsDomain.disabled = true;
});

async function loadData() {
    const response = await fetch('/portalData', {
        method: 'GET',
    });

    if (!response.ok) {
        throw new Error('Failed to fetch data');
    }

    const data = await response.json();
    pcscfSocket.value = data.pcscfSocket;
    imsDomain.value = data.imsDomain;
    populateTable(data.clients);
}

function stopEditing() {
    pcscfSocket.disabled = true;
    imsDomain.disabled = true;
    saveData.disabled = true;
    editData.textContent = editData.dataset.actionEdit;
}

editData.addEventListener('click', () => {
    if (editData.textContent === editData.dataset.actionEdit) {
        pcscfSocket.disabled = false;
        imsDomain.disabled = false;
        saveData.disabled = false;
        editData.textContent = editData.dataset.actionCancel;
    } else {
        stopEditing()
    }
})

saveData.addEventListener('click', async event => {
    event.preventDefault();

    const jsonData = {
        pcscfSocket: pcscfSocket.value,
        imsDomain: imsDomain.value
    };

    if (Object.values(jsonData).some(value => value === "")) {
        alert("All fields must be filled out");
        return;
    }

    saveData.disabled = true;

    const response = await fetch('/portalData', {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json'
        },
        body: JSON.stringify(jsonData)
    });

    if (response.ok) stopEditing();
    else {
        saveData.disabled = false;
        alert('Error: ' + response.statusText);
    }

});

recordForm.addEventListener('submit', async event => {
    event.preventDefault();

    const udpPortValue = parseInt(document.getElementById('udpPort').value, 10);
    if (isNaN(udpPortValue) || udpPortValue < 5000 || udpPortValue > 6000) {
        alert('UDP Port must be a number between 5000 and 6000');
        return;
    }

    const jsonData = {
        enabled: document.getElementById('enabled').value === 'true',
        imsi: document.getElementById('imsi').value,
        ki: document.getElementById('ki').value,
        opc: document.getElementById('opc').value,
        expires: document.getElementById('expires').value,
        udpPort: udpPortValue
    };

    if (Object.values(jsonData).some(value => value === "")) {
        alert("All fields must be filled out");
        return;
    }

    submitButton.disabled = true;

    try {
        const response = await fetch('/portal', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify(jsonData)
        });

        if (!response.ok) {
            throw new Error('Failed to fetch data');
        }

        const data = await response.json();
        populateTable(data);
    } catch (error) {
        alert('Error: ' + error.message);
        console.error('Error:', error);
    } finally {
        submitButton.disabled = false;
    }

    recordForm.reset();
});

function populateTable(data) {
    dataTable.innerHTML = '';
    data.forEach(record => {
        const newRow = dataTable.insertRow();

        const selectCell = newRow.insertCell(0);
        const selectCheckbox = document.createElement('input');
        selectCheckbox.type = 'checkbox';
        selectCell.appendChild(selectCheckbox);

        Object.entries(record).forEach(([key, value]) => {
            const newCell = newRow.insertCell();
            newCell.textContent = key === 'enabled' ? (value ? 'True' : 'False') : value;
        });

        const actionCell = newRow.insertCell();

        const editButton = document.createElement('button');
        editButton.classList.add('actions');
        editButton.textContent = "✏️";
        editButton.title = 'Edit';
        editButton.addEventListener('click', () => editRecord(newRow));

        const regButton = document.createElement('button');
        regButton.classList.add('actions');
        regButton.textContent = '⚡';
        regButton.title = 'Register';
        regButton.addEventListener('click', () => performRegister(newRow));

        const unRegButton = document.createElement('button');
        unRegButton.classList.add('actions');
        unRegButton.textContent = '⭕';
        unRegButton.title = 'Unregister';
        unRegButton.addEventListener('click', () => performRegister(newRow, true));

        const callButton = document.createElement('button');
        callButton.classList.add('actions');
        callButton.textContent = "📞";
        callButton.title = 'Call';
        callButton.addEventListener('click', () => performCall(newRow));

        const deleteButton = document.createElement('button');
        deleteButton.classList.add('actions');
        deleteButton.textContent = '❌';
        deleteButton.title = 'Delete';
        deleteButton.addEventListener('click', () => performDelete(newRow));

        actionCell.appendChild(editButton);
        actionCell.appendChild(regButton);
        actionCell.appendChild(unRegButton);
        actionCell.appendChild(callButton);
        actionCell.appendChild(deleteButton);
    });
}

deleteSelected.addEventListener('click', event => {
    event.preventDefault();
    const checkboxes = dataTable.querySelectorAll('input[type="checkbox"]:checked');

    if (checkboxes.length === 0) return;

    if (!confirm('Are you sure you want to remove all selected records?')) return;

    const imsilst = Array.from(checkboxes).map(chkbx => chkbx.closest('tr').cells[2].textContent);
    performDelete(imsilst);

    // checkboxes.forEach((checkbox) => {
    //     const row = checkbox.closest('tr');
    //     row.remove();
    // });
});

refresh.addEventListener('click', event => {
    event.preventDefault();
    loadData()
});


function editRecord(row) {
    recordForm.parentElement.open = true
    const cells = row.cells;
    document.getElementById('enabled').value = cells[1].textContent.toLowerCase();
    document.getElementById('imsi').value = cells[2].textContent;
    document.getElementById('ki').value = cells[3].textContent;
    document.getElementById('opc').value = cells[4].textContent;
    // document.getElementById('msisdn').value = cells[5].textContent;
    // document.getElementById('registration').value = cells[6].textContent;
    document.getElementById('expires').value = cells[7].textContent;
    document.getElementById('udpPort').value = cells[8].textContent;
    // row.remove();
}

async function deleteRecords(imsilst) {
    const response = await fetch('/portal', {
        method: 'DELETE',
        headers: {
            'Content-Type': 'application/json'
        },
        body: JSON.stringify(imsilst)
    });

    if (!response.ok) alert('Failed to delete UEs');
}

function performDelete(row) {
    if (!confirm('Are you sure you want to remove this record?')) return;
    const imsilst = [row.cells[2].textContent];
    deleteRecords(imsilst);
    loadData();
}


async function performRegister(row, unreg) {
    const params = { imsi: row.cells[2].textContent };
    const queryString = new URLSearchParams(params).toString();
    var url
    if (unreg) {
        url = `/unregister?${queryString}`;
    } else {
        url = `/register?${queryString}`;
    }

    const response = await fetch(url, {
        method: 'PUT',
    });

    if (!response.ok) alert('Error: ' + response.statusText);
}

async function performCall(row) {
    let num = row.cells[5].textContent
    if (isNaN(num)) num = row.cells[2].textContent
    let cdpn = prompt(`Enter CDPN to dial from UE (${num})`);
    if (!cdpn) return;
    const params = { imsi: row.cells[2].textContent, cdpn: cdpn };
    const queryString = new URLSearchParams(params).toString();
    const url = `/call?${queryString}`;

    const response = await fetch(url, {
        method: 'PUT',
    });
    if (!response.ok) alert('Error: ' + response.statusText);
}

btnRefreshCalls.addEventListener('click', async event => {
    btnRefreshCalls.disabled = true;

    try {
        const response = await fetch('/calls', { method: 'GET' });
        if (!response.ok) {
            throw new Error('Failed to fetch data');
        }
        const data = await response.json();
        populateCallsTable(data);
    } catch (error) {
        alert('Error: ' + error.message);
        console.error('Error:', error);
    } finally {
        btnRefreshCalls.disabled = false;
    }

});


btnClearCalls.addEventListener('click', () => { callsTable.innerHTML = '' });


function populateCallsTable(msgs) {
    callsTable.innerHTML = '';
    if (msgs) msgs.forEach(msg => populateCallsRecord(msg));
}

function populateCallsRecord(msg) {
    let row = Array.from(callsTable.rows).find(row => row.cells[0].textContent === msg.imsi && row.cells[5].textContent === msg.callID);

    if (row) {

        // 0 - <th>IMSI</th>
        // 1 - <th>MsIsdn</th>
        // 2 - <th>Start Time</th>
        // 3 - <th>End Time</th>
        // 4 - <th>Direction</th>
        // 5 - <th>Call ID</th>
        // 6 - <th>State</th>
        // 7 - <th>Action</th>

        row.cells[2].textContent = msg.startTime;
        row.cells[3].textContent = msg.endTime;
        row.cells[6].textContent = msg.state;

        if (msg.state.endsWith('ed')) {
            ringingSound.pause();
            ringingSound.currentTime = 0;

            let btnAnswer = row.getElementsByTagName('button')[0];
            btnAnswer.style.animation = '';
        }

        let btnHold = row.getElementsByTagName('button')[2];
        if (msg.callHold) btnHold.style.animation = animationProperty;
        else btnHold.style.animation = '';

        if (msg.endTime !== 'N/A') btnHold.style.animation = '';

        ws.send("Call record updated!");

        return
    }

    row = callsTable.insertRow();
    Object.entries(msg).forEach(([key, value]) => {
        if (key === 'flashAnswer' || key === 'callHold') return;
        const cell = row.insertCell();
        cell.textContent = value;
    });

    const actionCell = row.insertCell();

    const btn1 = document.createElement('button');
    btn1.classList.add('actions');
    btn1.textContent = "▶";
    btn1.title = 'Resume/Answer';
    btn1.addEventListener('click', () => actionRecord(row, btn1.title));

    const btn2 = document.createElement('button');
    btn2.classList.add('actions');
    btn2.textContent = "◼";
    btn2.title = 'Reject/Release';
    btn2.addEventListener('click', () => actionRecord(row, btn2.title));

    const btn3 = document.createElement('button');
    btn3.classList.add('actions');
    btn3.textContent = "✽";
    btn3.title = 'HoldCall';
    btn3.addEventListener('click', () => actionRecord(row, btn3.title));

    actionCell.appendChild(btn1);
    actionCell.appendChild(btn2);
    actionCell.appendChild(btn3);

    if (msg.flashAnswer) {
        btn1.style.animation = animationProperty;
        ringingSound.play()
    }

    ws.send("Call record added!");
}

ws.onopen = () => {
    console.log('Connected to server');
    // Send a message to the server
    ws.send("Hello, Server!")
};

ws.onmessage = (event) => {
    const msg = JSON.parse(event.data);

    if (msg.callID) {
        populateCallsRecord(msg)
        return
    }

    let row = Array.from(dataTable.rows).find(row => row.cells[2].textContent === msg.imsi);
    if (row) {
        const cells = row.cells;
        cells[5].textContent = msg.msisdn;
        cells[6].textContent = msg.regStatus;
        cells[7].textContent = msg.expires;

        ws.send("Line record updated!");
    }

};

async function actionRecord(row, action) {
    const params = { imsi: row.cells[0].textContent, callID: row.cells[5].textContent, action };
    const queryString = new URLSearchParams(params).toString();
    const url = `/callAction?${queryString}`;
    const response = await fetch(url, { method: 'POST' });
    if (!response.ok) alert('Error: ' + response.statusText);
}


ws.onclose = () => {
    console.log('Disconnected from server');
};

ws.onerror = (error) => {
    console.error('WebSocket Error:', error);
};