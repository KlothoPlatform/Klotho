[
	(import_declaration
		(import_spec_list
			[(import_spec
				(package_identifier)@package_id
				(interpreted_string_literal)@package_path
			)@spec
			(import_spec
				(interpreted_string_literal)@package_path
			)@spec
			]
		)@importList
	)@expression
	(import_declaration
			[(import_spec
				(package_identifier)@package_id
				(interpreted_string_literal)@package_path
			)@spec
			(import_spec
				(interpreted_string_literal)@package_path
			)@spec
			]
	)@expression
]