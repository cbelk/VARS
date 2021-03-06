//////////////////////////////////////////////////////////////////////////////////////
//                                                                                  //
//    VARS (Vulnerability Analysis Reference System) is software used to track      //
//    vulnerabilities from discovery through analysis to mitigation.                //
//    Copyright (C) 2017  Christian Belk                                            //
//                                                                                  //
//    This program is free software: you can redistribute it and/or modify          //
//    it under the terms of the GNU General Public License as published by          //
//    the Free Software Foundation, either version 3 of the License, or             //
//    (at your option) any later version.                                           //
//                                                                                  //
//    This program is distributed in the hope that it will be useful,               //
//    but WITHOUT ANY WARRANTY; without even the implied warranty of                //
//    MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the                 //
//    GNU General Public License for more details.                                  //
//                                                                                  //
//    See the full License here: https://github.com/cbelk/vars/blob/master/LICENSE  //
//                                                                                  //
//////////////////////////////////////////////////////////////////////////////////////

$(document).ready(function() {
	$('#report-list').change(function() {
		var report = $(this).val();
        if (report != "") {
            $.ajax({
                method  : 'GET',
                dataType: 'html',
                url     : '/report/'+report,
                success: function(data) {
                    $('#report-container').empty();
                    $('#report-container').html(data);
                },
                error: function() {
                    alert('Error loading report');
                }
            });
        }
	});
    $.ajax({
        method  : 'GET',
        dataType: 'json',
        url     : '/report/list',
        success: function(data) {
            for(i=0; i<data.length; i++) {
                $('#report-list').append('<option value="'+data[i]+'">'+data[i]+'</option>');
            }
        },
        error: function() {
            alert('Error loading report list');
        }
    });
});
