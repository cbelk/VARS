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

package vars

import (
	"database/sql"
	"encoding/json"
	"time"

	"github.com/lib/pq"
)

// VarsNullString holds a sql.NullString. Needed for marshaling/unmarshaling.
type VarsNullString struct {
	sql.NullString
}

// MarshalJSON will marshal the string if it is valid.
func (v VarsNullString) MarshalJSON() ([]byte, error) {
	if v.Valid {
		return json.Marshal(v.String)
	} else {
		return json.Marshal(nil)
	}
}

// UnmarshalJSON will unmarshal the string if it is valid and set valid to true, otherwise valid is set to false.
func (v *VarsNullString) UnmarshalJSON(data []byte) error {
	// Unmarshalling into a pointer will let us detect null
	var x *string
	if err := json.Unmarshal(data, &x); err != nil {
		return err
	}
	if x != nil {
		v.Valid = true
		v.String = *x
	} else {
		v.Valid = false
	}
	return nil
}

// VarsNullTime holds a pq.NullTime. Needed for marshaling/unmarshaling.
type VarsNullTime struct {
	pq.NullTime
}

// MarshalJSON will marshal the time if it is valid.
func (v VarsNullTime) MarshalJSON() ([]byte, error) {
	if v.Valid {
		t, err := v.Value()
		if err != nil {
			return make([]byte, 0), err
		}
		return json.Marshal(t)
	} else {
		return json.Marshal(nil)
	}
}

// UnmarshalJSON will unmarshal the time if it is valid and set valid to true, otherwise valid is set to false.
func (v *VarsNullTime) UnmarshalJSON(data []byte) error {
	var x time.Time
	if err := json.Unmarshal(data, &x); err != nil {
		return err
	}
	if !x.IsZero() {
		v.Valid = true
		v.Time = x
	} else {
		v.Valid = false
		v.Time = x
	}
	return nil
}

// VarsNullBool holds a sql.NullBool. Needed for marshaling/unmarshaling.
type VarsNullBool struct {
	sql.NullBool
}

// MarshalJSON will marshal the string if it is valid.
func (v VarsNullBool) MarshalJSON() ([]byte, error) {
	if v.Valid {
		return json.Marshal(v.Bool)
	} else {
		return json.Marshal(nil)
	}
}

// UnmarshalJSON will unmarshal the string if it is valid and set valid to true, otherwise valid is set to false.
func (v *VarsNullBool) UnmarshalJSON(data []byte) error {
	// Unmarshalling into a pointer will let us detect null
	var x *bool
	if err := json.Unmarshal(data, &x); err != nil {
		return err
	}
	if x != nil {
		v.Valid = true
		v.Bool = *x
	} else {
		v.Valid = false
	}
	return nil
}
