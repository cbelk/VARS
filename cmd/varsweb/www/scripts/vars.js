function deleteSubmitHandlers() {
    $('.submit-cve').off('submit');
}

function hideAlerts() {
    $('#vuln-modal-alert-success').hide();
    $('#vuln-modal-alert-danger').hide();
    $('#vuln-modal-alert-warning').hide();
    $('#vuln-modal-alert-warning-item').text('');
}

function hideModalEditSubmit() {
    $('.submit-cve').hide();
}

function hideModalEditDivs() {
    $('#vuln-modal-div-edit-summary').hide();
    $('#vuln-modal-div-add-cve').hide();
    $('#vuln-modal-div-edit-cvss').hide();
    $('#vuln-modal-div-edit-corpscore').hide();
    $('.edit-cve-input').attr('readonly');
    $('.edit-cve-input').addClass('form-control-plaintext');
    $('.edit-cve-input').removeClass('form-control');
}

function handleModalAddItem(btnID) {
    switch(btnID) {
        case 'vuln-modal-add-cve':
            $('#vuln-modal-div-add-cve').show();
            break;
    }
}

function showModalEditDiv(btnID, num) {
    hideModalEditDivs();
    hideModalEditSubmit();
    switch(btnID) {
        case 'vuln-modal-edit-summary':
            $('#vuln-modal-div-summary').hide();
            $('#vuln-modal-div-edit-summary').show();
            break;
        case 'cve':
            $('#vuln-modal-edit-cve-'+num).removeAttr('readonly');
            $('#vuln-modal-edit-cve-'+num).removeClass('form-control-plaintext');
            $('#vuln-modal-edit-cve-'+num).addClass('form-control');
            $('#vuln-modal-edit-cve-'+num+'-submit').show();
            $('#vuln-modal-edit-cve-'+num+'-btn').hide();
            break;
        case 'vuln-modal-edit-cvss':
            $('#vuln-modal-div-cvss').hide();
            $('#vuln-modal-div-edit-cvss').show();
            break;
        case 'vuln-modal-edit-corpscore':
            $('#vuln-modal-div-corpscore').hide();
            $('#vuln-modal-div-edit-corpscore').show();
            break;
    }
    hideAlerts();
}

function showModalDelete(btnID, num) {
    switch(btnID) {
        case 'cve':
            var cve = $('#vuln-modal-edit-cve-'+num).attr('data-original');
            $('#vuln-modal-alert-warning-item').text('Delete '+cve+'?');
            $('#vuln-modal-alert-warning').show();
            $('#vuln-modal-warning-yes').attr('onclick', 'handlePromptChoice("cve", "yes", "'+cve+'", "'+num+'")');
            $('#vuln-modal-warning-no').attr('onclick', 'handlePromptChoice("cve", "no", "'+cve+'", "'+num+'")');
            break;
    }
}

function handlePromptChoice(btnId, choice, item, itemID) {
    switch(btnId) {
        case 'cve':
            if (choice == 'yes') {
                var vid = $('#vuln-modal-vulnid').text();
                $.ajax({
                    method : 'DELETE',
                    url    : '/vulnerability/'+vid+'/cve/'+item,
                    success: function(data) {
                        hideModalEditDivs();
                        $('#vuln-modal-alert-success').show();
                        $('#vuln-modal-div-cve-'+itemID).hide();
                    },
                    error: function() {
                        $('#vuln-modal-alert-danger').show();
                    }
                });
            }
            $('#vuln-modal-alert-warning-item').text('');
            $('#vuln-modal-alert-warning').hide();
            $('#vuln-modal-warning-yes').attr('onclick', 'placeholder()');
            $('#vuln-modal-warning-no').attr('onclick', 'placeholder()');
            break;
    }
}

function appendCve(cve, num) {
    $('#vuln-modal-cve-list').append('<div id="vuln-modal-div-cve-'+num+'"> <div class="col-1"> <div class="btn-group" role="group"><button type="button" class="btn-sm bg-white text-success border-0 vme-btn vme-btn-cve" id="vuln-modal-edit-cve-' + num + '-btn" data-edit-btn-group="cve" onclick="showModalEditDiv(\'cve\','+num+')" aria-label="Edit"> <span aria-hidden="true">&#9998;</span> </button> <button type="button" class="btn-sm bg-white text-danger border-0 vme-btn vme-btn-cve" id="vuln-modal-delete-cve-' + num + '-btn" data-delete-btn-group="cve" onclick="showModalDelete(\'cve\','+num+')" aria-label="Delete"> <span aria-hidden="true">&times;</span> </button></div> </div> <div class="col-11"> <form class="form-inline" id="vuln-modal-form-cve-'+num+'"> <input type="text" class="form-control-plaintext edit-cve-input" readonly id="vuln-modal-edit-cve-' + num + '"value="' + cve + '" name="cve" data-original="'+cve+'"><button type="submit" class="btn btn-dark submit-cve" id="vuln-modal-edit-cve-'+num+'-submit">Submit</button></form></div></div>');
    $('#vuln-modal-form-cve-'+num).on('submit', {cveid: num}, function(event) {
        event.preventDefault();
        var cveid = event.data.cveid;
        var fdata = $('#vuln-modal-edit-cve-'+cveid).serialize();
        var vid = $('#vuln-modal-vulnid').text();
        var cveString = $('#vuln-modal-edit-cve-'+cveid).attr('data-original');
        $.ajax({
            method : 'POST',
            url    : '/vulnerability/'+vid+'/cve/'+cveString,
            data   : fdata,
            success: function(data) {
                hideModalEditDivs();
                $('#vuln-modal-alert-success').show();
                $('#vuln-modal-edit-cve-'+cveid+'-submit').hide();
                $('#vuln-modal-edit-cve-'+cveid+'-btn').show();
            },
            error: function() {
                $('#vuln-modal-alert-danger').show();
            }
        });
    });
}

function updateVulnModal(vuln, modal) {
    modal.find('.modal-title').text(vuln.Name);
    modal.find('#vuln-modal-vulnid').text(vuln.ID);
    // Summary
    modal.find('#vuln-modal-summary').text(vuln.Summary);
    modal.find('#vuln-modal-summary-edit').val(vuln.Summary);
    modal.find('#vuln-modal-form-summary').attr('action', '/vulnerability/' + vuln.ID + '/summary');
    //CVEs
    modal.find('#vuln-modal-cve-list').empty();
    if (vuln.Cves != null) {
        vuln.Cves.sort();
        for (i = 0; i < vuln.Cves.length; i++) {
            appendCve(vuln.Cves[i], i);
        }
    }
    hideModalEditSubmit();
    // Cvss
    modal.find('#vuln-modal-cvss').text(vuln.Cvss);
    modal.find('#vuln-modal-cvss-edit').attr('value', vuln.Cvss);
    if (vuln.CvssLink == null) {
        modal.find('#vuln-modal-cvss-link').attr('href', 'https://www.first.org/cvss/calculator/3.0');
    } else {
        modal.find('#vuln-modal-cvss-link').attr('href', vuln.CvssLink);
        modal.find('#vuln-modal-cvss-link-edit').attr('value', vuln.CvssLink);
    }
    // CorpScore
    modal.find('#vuln-modal-corpscore').text(vuln.CorpScore);
    modal.find('#vuln-modal-corpscore-edit').val(vuln.CorpScore);
    // Test
    modal.find('#vuln-modal-test').text(vuln.Test);
    // Mitigation
    modal.find('#vuln-modal-mitigation').text(vuln.Mitigation);
    // Initiated
    modal.find('#vuln-modal-initiated').text(vuln.Dates.Initiated);
    if (vuln.Dates.Mitigated == null) {
        modal.find('#vuln-modal-mitigated').text('');
    } else {
        modal.find('#vuln-modal-mitigated').text(vuln.Dates.Mitigated);
    }
    modal.find('#vuln-modal-tickets-list').empty();
    if (vuln.Tickets != null) {
        vuln.Tickets.sort();
        for (i = 0; i < vuln.Tickets.length; i++) {
            modal.find('#vuln-modal-tickets-list').append('<div class="col-1"> <button type="button" class="btn-sm bg-white text-success border-0 vme-btn vme-btn-tickets" id="vuln-modal-edit-ticket-' + i + '-btn" aria-label="Edit"> <span aria-hidden="true">&#9998;</span> </button> </div> <div class="col-11"> <p id="vuln-modal-edit-ticket-' + i + '">' + vuln.Tickets[i]  + '</p></div>');
        }
    }
    modal.find('#vuln-modal-ref-list').empty();
    for (i = 0; i < vuln.References.length; i++) {
        modal.find('#vuln-modal-ref-list').append('<div class="col-1"> <button type="button" class="btn-sm bg-white text-success border-0 vme-btn vme-btn-refs" id="vuln-modal-edit-ref-' + i + '-btn" aria-label="Edit"> <span aria-hidden="true">&#9998;</span> </button> </div> <div class="col-11"> <a id="vuln-modal-edit-ref-' + i + '" href="' + vuln.References[i] + '" class="text-primary">' + vuln.References[i]  + '</a></div>');
    }
    if (vuln.Exploitable == null) {
        modal.find('#vuln-modal-exploitable').text('false');
    } else {
        modal.find('#vuln-modal-exploitable').text(vuln.Exploitable);
    }
    if (vuln.Exploit == null) {
        modal.find('#vuln-modal-exploit').text('');
    } else {
        modal.find('#vuln-modal-exploit').text(vuln.Exploit);
    }
    modal.find('#vuln-modal-affected-table').empty();
    for (i = 0; i < vuln.AffSystems.length; i++) {
        modal.find('#vuln-modal-affected-table').append('<tr><td>' + vuln.AffSystems[i].Sys.Name + '</td><td>' + vuln.AffSystems[i].Sys.Description + '</td><td>'+ vuln.AffSystems[i].Sys.Location + '</td><td>'+ vuln.AffSystems[i].Sys.State + '</td><td>'+ vuln.AffSystems[i].Mitigated + '</td></li>')
    }
}

$('#vuln-modal').on('show.bs.modal', function (event) {
    // Get vulnid
    var row = $(event.relatedTarget);
    var vid = row.data('vid');
    var modal = $(this);

    //Get data from server
    var req = new XMLHttpRequest();
    req.onreadystatechange = function() {
        if(this.readyState == 4 && this.status == 200) {
            var vuln = JSON.parse(this.responseText);
            updateVulnModal(vuln, modal);
            hideModalEditDivs();
            hideModalEditSubmit();
            hideAlerts();
            $('.vme-btn').hide();
        }
    };
    req.open('GET', '/vulnerability/' + vid, true);
    req.send();
});

$('#vuln-modal').on('hidden.bs.modal', function (event) {
    hideModalEditDivs();
    hideModalEditSubmit();
    hideAlerts();
    $('#vuln-modal-vulnid').text('-1');
    $('#vuln-modal-affected-collapse').collapse('hide');
    $('#vuln-modal-div-summary').show();
    $('#vuln-modal-summary-edit').val('');
    $('#vuln-modal-div-cvss').show();
    $('#vuln-modal-cvss-edit').attr('value', '');
    $('#vuln-modal-cvss-link-edit').attr('value', '');
});

$('#vuln-modal-section-summary').hover(function() {
        $('.vme-btn-summary').show();
    }, function() {
        $('.vme-btn-summary').hide();
    }
);

$('#vuln-modal-section-cve').hover(function() {
        $('.vme-btn-cve').show();
    }, function() {
        $('.vme-btn-cve').hide();
    }
);

$('#vuln-modal-section-cvss').hover(function() {
        $('.vme-btn-cvss').show();
    }, function() {
        $('.vme-btn-cvss').hide();
    }
);

$('#vuln-modal-section-corpscore').hover(function() {
        $('.vme-btn-corpscore').show();
    }, function() {
        $('.vme-btn-corpscore').hide();
    }
);

$('#vuln-modal-section-test').hover(function() {
        $('.vme-btn-test').show();
    }, function() {
        $('.vme-btn-test').hide();
    }
);

$('#vuln-modal-section-mitigation').hover(function() {
        $('.vme-btn-mitigation').show();
    }, function() {
        $('.vme-btn-mitigation').hide();
    }
);

$('#vuln-modal-section-tickets').hover(function() {
        $('.vme-btn-tickets').show();
    }, function() {
        $('.vme-btn-tickets').hide();
    }
);

$('#vuln-modal-section-refs').hover(function() {
        $('.vme-btn-refs').show();
    }, function() {
        $('.vme-btn-refs').hide();
    }
);

$('#vuln-modal-section-exploitable').hover(function() {
        $('.vme-btn-exploitable').show();
    }, function() {
        $('.vme-btn-exploitable').hide();
    }
);

$('#vuln-modal-section-exploit').hover(function() {
        $('.vme-btn-exploit').show();
    }, function() {
        $('.vme-btn-exploit').hide();
    }
);

$(document).ready(function() {
	$('#vuln-modal-form-summary').on('submit', function(event) {
		event.preventDefault();
		var fdata = $('#vuln-modal-summary-edit').serialize();
		var vid = $('#vuln-modal-vulnid').text();
		var summary = $('#vuln-modal-summary-edit').val();
		$.ajax({
			method : 'POST',
			url    : $('#vuln-modal-form-summary').attr('action'),
			data   : fdata,
			success: function(data) {
				$('#vuln-modal-summary').text(summary);
				$('#vuln-modal-div-summary').show();
				$('#vuln-modal-div-edit-summary').hide();
				$('#vuln-modal-alert-success').show();
				$("tr[data-vid='"+vid+"']").find("td:eq(1)").text(summary);
			},
            error: function() {
                $('#vuln-modal-alert-danger').show();
            }
		});
	});
	$('#vuln-modal-form-cvss').on('submit', function(event) {
		event.preventDefault();
		var fdata = $('#vuln-modal-form-cvss').serialize();
		var vid = $('#vuln-modal-vulnid').text();
		var cvssScore = $('#vuln-modal-cvss-edit').val();
		var cvssLink = $('#vuln-modal-cvss-link-edit').val();
		$.ajax({
			method : 'POST',
			url    : '/vulnerability/'+vid+'/cvss',
			data   : fdata,
			success: function(data) {
				$('#vuln-modal-cvss').text(cvssScore);
				$('#vuln-modal-cvss').attr('href', cvssLink);
				$('#vuln-modal-div-cvss').show();
				$('#vuln-modal-div-edit-cvss').hide();
				$('#vuln-modal-alert-success').show();
				$("tr[data-vid='"+vid+"']").find("td:eq(2)").text(cvssScore);
			},
            error: function() {
                $('#vuln-modal-alert-danger').show();
            }
		});
	});
	$('#vuln-modal-form-corpscore').on('submit', function(event) {
		event.preventDefault();
		var fdata = $('#vuln-modal-form-corpscore').serialize();
		var vid = $('#vuln-modal-vulnid').text();
		var corpscore = $('#vuln-modal-corpscore-edit').val();
		$.ajax({
			method : 'POST',
			url    : '/vulnerability/'+vid+'/corpscore',
			data   : fdata,
			success: function(data) {
				$('#vuln-modal-corpscore').text(corpscore);
				$('#vuln-modal-div-corpscore').show();
				$('#vuln-modal-div-edit-corpscore').hide();
				$('#vuln-modal-alert-success').show();
				$("tr[data-vid='"+vid+"']").find("td:eq(3)").text(corpscore);
			},
            error: function() {
                $('#vuln-modal-alert-danger').show();
            }
		});
	});
	$('#vuln-modal-form-add-cve').on('submit', function(event) {
		event.preventDefault();
		var fdata = $('#vuln-modal-form-add-cve').serialize();
		var vid = $('#vuln-modal-vulnid').text();
		var cve = $('#vuln-modal-add-cve-text').val();
		$.ajax({
			method : 'PUT',
			url    : '/vulnerability/'+vid+'/cve',
			data   : fdata,
			success: function(data) {
				$('#vuln-modal-div-add-cve').hide();
				$('#vuln-modal-alert-success').show();
                var cveID = $('#vuln-modal-cve-list').children().length - 1;
                appendCve(cve, cveID);
                hideModalEditSubmit();
			},
            error: function() {
                $('#vuln-modal-alert-danger').show();
            }
		});
	});
});